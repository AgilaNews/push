package devicemapper

import (
	"encoding/json"
	"errors"
	"gopkg.in/redis.v4"
	"sync"
)

const (
	TOKEN_PREFIX    = "PUSH_TOKEN_"
	DEVICEID_PREFIX = "PUSH_DEVICE_ID_"
)

type RedisDeviceMapper struct {
	redis_client *redis.Client

	deviceLock sync.RWMutex
}

func NewRedisDeviceMapper(addr string) (*RedisDeviceMapper, error) {
	mapper := &RedisDeviceMapper{}
	/*
		mapper.redis_client = redis.NewFailoverClient(
			&redis.FailoverOptions{
				MasterName:    "master",
				SentinelAddrs: []string{addr},
				MaxRetries:    3,
			},
		)
	*/
	mapper.redis_client = redis.NewClient(
		&redis.Options{
			Addr: addr,
		})

	if _, err := mapper.redis_client.Ping().Result(); err != nil {
		return nil, err
	} else {
		return mapper, nil
	}
}

func (dm *RedisDeviceMapper) AddNewDevice(device *Device) error {
	dm.deviceLock.Lock()
	defer dm.deviceLock.Unlock()
	if val, err := json.Marshal(device); err != nil {
		return err
	} else {
		saved := string(val)
		pipeline := dm.redis_client.Pipeline()
		pipeline.HSet(TOKEN_PREFIX, device.Token, saved)
		pipeline.HSet(DEVICEID_PREFIX, device.DeviceId, saved)
		if _, err := pipeline.Exec(); err != redis.Nil {
			return errors.New("add device error")
		}
	}

	return nil
}

func (dm *RedisDeviceMapper) RemoveDevice(device *Device) error {
	dm.deviceLock.Lock()
	defer dm.deviceLock.Unlock()
	pipeline := dm.redis_client.Pipeline()
	pipeline.HDel(TOKEN_PREFIX, device.Token)
	pipeline.HDel(DEVICEID_PREFIX, device.DeviceId)
	if _, err := pipeline.Exec(); err != redis.Nil {
		return errors.New("add device error")
	}

	return nil
}

func (dm *RedisDeviceMapper) GetDeviceByToken(token string) (*Device, error) {
	return dm.getDeviceFromRedis(TOKEN_PREFIX, token)
}

func (dm *RedisDeviceMapper) GetDeviceById(device_id string) (*Device, error) {
	return dm.getDeviceFromRedis(DEVICEID_PREFIX, device_id)
}

func (dm *RedisDeviceMapper) GetAllDevice() ([]*Device, error) {
	dm.deviceLock.RLock()
	defer dm.deviceLock.RUnlock()
	ret := dm.redis_client.HGetAll(DEVICEID_PREFIX)
	if ret.Err() != nil {
		return nil, errors.New("get all device from redis error")
	}

	val := ret.Val()
	devices := make([]*Device, len(val))
	idx := 0
	for _, v := range val {
		device := &Device{}
		if err := json.Unmarshal([]byte(v), device); err != nil {
			continue
		} else {
			devices[idx] = device
			idx++
		}
	}

	return devices, nil
}

func (dm *RedisDeviceMapper) getDeviceFromRedis(key, id string) (*Device, error) {
	dm.deviceLock.RLock()
	defer dm.deviceLock.RUnlock()
	if ok := dm.redis_client.HExists(key, id).Val(); ok {
		device := &Device{}
		if v, err := dm.redis_client.HGet(key, id).Bytes(); err != nil {
			return nil, errors.New("get bytes from redis error")
		} else {
			if err := json.Unmarshal(v, device); err != nil {
				return nil, errors.New("get value of device error")
			}
			return device, nil
		}

	}

	return nil, nil

}
