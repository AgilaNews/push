package env

import (
	"encoding/json"
	"fcm/devicemapper"
	"fmt"
	"os"
	"time"

	"database/sql"
	"github.com/alecthomas/log4go"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
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
		Addr string `json:"addr"`
	} `json:"http_server"`
	Mysql struct {
		Read  MysqlConfiguration `json:"read"`
		Write MysqlConfiguration `json:"write"`
	} `json:"mysql"`
	Location string `json:"location"`
}

var (
	DeviceMapper devicemapper.DeviceMapper
	Config       *Configuration
	level_map    = map[string]log4go.Level{
		"DEBUG": log4go.DEBUG,
		"INFO":  log4go.INFO,
		"ERROR": log4go.ERROR,
	}

	Rdb      *sql.DB
	Wdb      *sql.DB
	Location *time.Location
)

func (mysqlConfig *MysqlConfiguration) getConnection() (*sql.DB, error) {
	config := mysql.Config{
		User:   mysqlConfig.User,
		Passwd: mysqlConfig.Password,
		Net:    "tcp",
		Addr:   fmt.Sprintf("%s:%s", mysqlConfig.Host, mysqlConfig.Port),
		DBName: mysqlConfig.DB,
		Params: map[string]string{
			"charset": "utf8mb4,utf8",
		},
		Collation: "utf8_general_ci",
	}

	db, err := sql.Open("mysql", config.FormatDSN())

	if err != nil {
		return nil, err
	}

	if mysqlConfig.PoolSize > 0 {
		db.SetMaxIdleConns(mysqlConfig.PoolSize)
		db.SetMaxOpenConns(mysqlConfig.PoolSize)
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
	if DeviceMapper, err = devicemapper.NewRedisDeviceMapper(Config.Redis.Addr); err != nil {
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
	if Rdb, err = Config.Mysql.Read.getConnection(); err != nil {
		log4go.Global.Error("init read db error")
		return err
	}
	if Wdb, err = Config.Mysql.Write.getConnection(); err != nil {
		log4go.Global.Error("init write db error")
		return err
	}

	log4go.Global.Info("env init success [%v]", value)
	return nil
}
