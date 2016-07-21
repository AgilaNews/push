package env

import (
	"fcm/devicemapper"
	"github.com/alecthomas/log4go"
)

var (
	Logger       log4go.Logger
	DeviceMapper devicemapper.DeviceMapper
)

func Init() error {
	Logger = make(log4go.Logger)
	Logger.AddFilter("stdout", log4go.DEBUG, log4go.NewConsoleLogWriter())
	Logger.AddFilter("log", log4go.FINE, log4go.NewFileLogWriter("logs/push.log", true))

	var err error
	if DeviceMapper, err = devicemapper.NewRedisDeviceMapper("0.0.0.0:6379"); err != nil {
		return err
	}
	return nil
}
