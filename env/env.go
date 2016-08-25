package env

import (
	"encoding/json"
	"fmt"
	"os"
	"push/device"
	"push/fcm"
	"push/task"
	"time"

	"github.com/alecthomas/log4go"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

type MysqlConfiguration struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DB       string `json:"db"`
	User     string `json:"user"`
	Password string `json:"password"`
	Charset  string `json:"charset"`
	PoolSize int    `json:"pool"`
}

type Configuration struct {
	Log struct {
		Path    string `json:"path"`
		Level   string `json:"level"`
		Console bool   `json:"console"`
	} `json:"log"`
	Redis struct {
		Addr string `json:"addr"`
	} `json:"redis"`
	AppServer struct {
		SenderId    string `json:"sender_id"`
		SecurityKey string `json:"security_key"`
	} `json:"app_server"`
	HttpServer struct {
		Addr        string `json:"addr"`
		SwaggerPath string `json:"swagger_path"`
	} `json:"http_server"`
	Mysql struct {
		Read  MysqlConfiguration `json:"read"`
		Write MysqlConfiguration `json:"write"`
	} `json:"mysql"`
	Location string `json:"location"`
}

var (
	Wdb       *gorm.DB
	Rdb       *gorm.DB
	Config    *Configuration
	level_map = map[string]log4go.Level{
		"DEBUG": log4go.DEBUG,
		"INFO":  log4go.INFO,
		"ERROR": log4go.ERROR,
	}
	Location *time.Location
)

func (mysqlConfig *MysqlConfiguration) getConnection() (*gorm.DB, error) {
	connstr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local&collation=%s",
		mysqlConfig.User,
		mysqlConfig.Password,
		mysqlConfig.Host,
		mysqlConfig.Port,
		mysqlConfig.DB,
		"utf8mb4",
		"utf8_general_ci",
	)

	db, err := gorm.Open("mysql", connstr)
	if err != nil {
		return nil, err
	}

	if mysqlConfig.PoolSize > 0 {
		db.DB().SetMaxIdleConns(mysqlConfig.PoolSize)
		db.DB().SetMaxOpenConns(mysqlConfig.PoolSize)
	}

	err = db.DB().Ping()

	if err != nil {
		return nil, err
	}

	return db, nil
}

func Init() error {
	value := os.Getenv("RUN_ENV")
	var conffile string
	switch value {
	case "rd":
		conffile = "config.json.rd"
	case "sandbox":
		conffile = "config.json.sandbox"
	default:
		conffile = "config.json.online"
	}
	conffile = "./conf/" + conffile
	fmt.Println("loading config file: " + conffile)

	file, err := os.Open(conffile)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(file)
	Config = &Configuration{}
	if err = decoder.Decode(Config); err != nil {
		return err
	}

	//init log
	var level log4go.Level
	var ok bool
	if level, ok = level_map[Config.Log.Level]; !ok {
		level = log4go.INFO
	}

	if Config.Log.Console {
		log4go.Global.AddFilter("stdout", level, log4go.NewConsoleLogWriter())
	}
	log4go.Global.AddFilter("log", level, log4go.NewFileLogWriter(Config.Log.Path, false))

	// init device mapper
	// use full when you want to push to certain device
	if device.GlobalDeviceMapper, err = device.NewRedisDeviceMapper(Config.Redis.Addr); err != nil {
		log4go.Global.Info("init device mapper fail")
		return err
	}

	if Config.Location == "" {
		Location, err = time.LoadLocation(Config.Location)
		if err != nil {
			log4go.Global.Error("load location %v error", Config.Location)
			return err
		}
	}

	//init mysql
	if Wdb, err = Config.Mysql.Write.getConnection(); err != nil {
		log4go.Global.Error("init write db error")
		return err
	}

	Wdb.AutoMigrate(&fcm.PushModel{}, &task.Task{})
	if err := Wdb.Model(&task.Task{}).AddUniqueIndex("idx_source_and_uid", "source", "uid").Error; err != nil {
		return err
	}

	if Rdb, err = Config.Mysql.Read.getConnection(); err != nil {
		log4go.Global.Error("init read db error")
		return err
	}

	if task.GlobalTaskManager, err = task.NewTaskManager(Rdb, Wdb); err != nil {
		log4go.Global.Error("new task manager")
		return err
	}

	if fcm.GlobalAppServer, err = fcm.NewAppServer(Config.AppServer.SenderId, Config.AppServer.SecurityKey); err != nil {
		log4go.Global.Error("new app server")
		return err
	}

	if fcm.GlobalPushManager, err = fcm.NewPushManager(task.GlobalTaskManager, Rdb, Wdb); err != nil {
		log4go.Global.Error("new push manager error")
		return err
	}

	task.GlobalTaskManager.RegisterTaskSourceHandler(task.TASK_SOURCE_PUSH, fcm.GlobalPushManager)

	log4go.Global.Info("env init success [%v]", value)
	return nil
}
