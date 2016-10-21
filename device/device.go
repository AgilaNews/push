package device

import (
	"fmt"
)

var (
	GlobalDeviceMapper DeviceMapper

	ErrDeviceNotFound = fmt.Errorf("device not found")
)

type Device struct {
	ID            string `gorm:"column:id;primary_key;" json:"-"`
	Token         string `gorm:"column:token;type:varchar(1024);index;not null;" json:"token"`
	UserId        string `gorm:"column:user_id;type:varchar(64);index;default:'';" json:"user_id"`
	DeviceId      string `gorm:"column:device_id;type:varchar(64);index;not null" json:"device_id"`
	ClientVersion string `gorm:"column:client_version;type:varchar(32);index;" json:"client_version"`
	Imsi          string `gorm:"column:imsi;type:varchar(64);" json:"imsi"`
	Os            string `gorm:"column:os;type:varchar(32);index;" json:"os"`
	OsVersion     string `gorm:"column:os_version;type:varchar(32);" json:"os_version"`
	Vendor        string `gorm:"vendor:vendor;type:varchar(32); "json:"vendor"`

	Slice int `gorm:"-"`
}

type DeviceMapper interface {
	AddNewDevice(device *Device) error
	RemoveDevice(device *Device) error
	GetDeviceByToken(token string) (*Device, error)
	GetDeviceById(device_id string) (*Device, error)
	GetDevicesById(device_ids []string) ([]*Device, error)
	GetDeviceByUserId(user_id string) ([]*Device, error)
	GetAllDevice() ([]*Device, error)
}
