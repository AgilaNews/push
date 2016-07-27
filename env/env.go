package env

import (
	"encoding/json"
	"fcm/devicemapper"
	"fmt"
	"os"

	"github.com/alecthomas/log4go"
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

	var level log4go.Level
	var ok bool
	if level, ok = level_map[Config.Log.Level]; !ok {
		level = log4go.INFO
	}

	if Config.Log.Console {
		log4go.Global.AddFilter("stdout", level, log4go.NewConsoleLogWriter())
	}

	log4go.Global.AddFilter("log", level, log4go.NewFileLogWriter(Config.Log.Path, false))

	if DeviceMapper, err = devicemapper.NewRedisDeviceMapper(Config.Redis.Addr); err != nil {
		log4go.Global.Info("init device mapper fail")
		return err
	}

	log4go.Global.Info("env init success [%v]", value)
	return nil
}
