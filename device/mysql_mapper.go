package device

import (
	"fmt"

	"github.com/alecthomas/log4go"
	"github.com/jinzhu/gorm"
)

func (d Device) TableName() string {
	return "tb_device"
}

type MysqlDeviceMapper struct {
	Wdb, Rdb *gorm.DB
}

func NewMysqlDeviceMapper(rdb, wdb *gorm.DB) (*MysqlDeviceMapper, error) {
	return &MysqlDeviceMapper{
		Wdb: wdb,
		Rdb: rdb,
	}, nil
}

func (dm *MysqlDeviceMapper) AddNewDevice(device *Device) error {
	return nil
}

func (dm *MysqlDeviceMapper) RemoveDevice(device *Device) error {
	if len(device.ID) == 0 {
		return fmt.Errorf("please set device ID")
	}

	return dm.Wdb.Delete(device).Error
}

func (dm *MysqlDeviceMapper) GetDeviceByToken(token string) (*Device, error) {
	d := Device{}
	if err := dm.Rdb.Where("token = ?", token).First(&d).Error; err != nil {
		return nil, err
	}

	return &d, nil
}

func (dm *MysqlDeviceMapper) GetDeviceById(device_id string) (*Device, error) {
	d := Device{}
	if ret := dm.Rdb.Where("device_id = ?", device_id).First(&d); ret.Error != nil {
		if ret.RecordNotFound() {
			return nil, ErrDeviceNotFound
		} else {
			return nil, ret.Error
		}
	}

	return &d, nil
}

func (dm *MysqlDeviceMapper) GetDeviceByUserId(user_id string) ([]*Device, error) {
	d := []*Device{}
	if ret := dm.Rdb.Where("user_id = ?", user_id).Find(&d); ret.Error != nil {
		if ret.RecordNotFound() {
			return nil, ErrDeviceNotFound
		} else {
			return nil, ret.Error
		}
	}

	return d, nil
}

func (dm *MysqlDeviceMapper) GetDevicesById(device_ids []string) ([]*Device, error) {
	devices := []*Device{}
	ret := make([]*Device, len(device_ids))

	if ret := dm.Rdb.Where("device_id IN (?)", device_ids).Find(&devices); ret.Error != nil {
		log4go.Warn("read sql error: %v", ret.Error)
		return nil, ret.Error
	}

	m := make(map[string]*Device)
	for _, device := range devices {
		m[device.DeviceId] = device
	}

	for i := 0; i < len(device_ids); i++ {
		if d, ok := m[device_ids[i]]; ok {
			ret[i] = d
		} else {
			ret[i] = nil
		}
	}

	return ret, nil
}

func (dm *MysqlDeviceMapper) GetAllDevice() ([]*Device, error) {
	ret := []*Device{}

	if err := dm.Rdb.Find(&ret).Error; err != nil {
		return nil, err
	}

	return ret, nil
}
