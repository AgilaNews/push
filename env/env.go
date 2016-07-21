package env

import (
	"encoding/json"
	"fcm/devicemapper"
	"github.com/alecthomas/log4go"
	"os"
    "fmt"
)

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
}

var (
	Logger       log4go.Logger
	DeviceMapper devicemapper.DeviceMapper
	Config       *Configuration

	level_map = map[string]log4go.Level{
		"DEBUG": log4go.DEBUG,
		"INFO":  log4go.INFO,
		"ERROR": log4go.ERROR,
	}
)

func Init() error {
    value := os.Getenv("RUN_ENV")
    var conffile string
    switch value {
        case "rd":
            conffile = "config.json.rd"
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

	var level log4go.Level
	var ok bool
	if level, ok = level_map[Config.Log.Level]; !ok {
		level = log4go.INFO
	}
	Logger = make(log4go.Logger)
	if Config.Log.Console {
		Logger.AddFilter("stdout", level, log4go.NewConsoleLogWriter())
	}

	Logger.AddFilter("log", level, log4go.NewFileLogWriter(Config.Log.Path, false))

	if DeviceMapper, err = devicemapper.NewRedisDeviceMapper(Config.Redis.Addr); err != nil {
		Logger.Info("init device mapper fail")
		return err
	}
	Logger.Info("env init success")
	return nil
}
