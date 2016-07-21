package env

import (
	"encoding/json"
	"fcm/devicemapper"
	"github.com/alecthomas/log4go"
	"os"
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
)

func Init() error {
	file, err := os.Open("conf/config.json")
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(file)
	Config = &Configuration{}
	if err = decoder.Decode(Config); err != nil {
		return err
	}

	Logger = make(log4go.Logger)
	if Config.Log.Console {
		Logger.AddFilter("stdout", log4go.DEBUG, log4go.NewConsoleLogWriter())
	}
	Logger.AddFilter("log", log4go.FINE, log4go.NewFileLogWriter(Config.Log.Path, true))

	if DeviceMapper, err = devicemapper.NewRedisDeviceMapper(Config.Redis.Addr); err != nil {
		Logger.Info("init device mapper fail")
		return err
	}
	Logger.Info("env init success")
	return nil
}
