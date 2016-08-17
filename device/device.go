package device

var (
	GlobalDeviceMapper DeviceMapper
)

type Device struct {
	Token         string `json:"token"`
	DeviceId      string `json:"device_id"`
	ClientVersion string `json:"client_version"`
	Imsi          string `json:"imsi"`
	Os            string `json:"os"`
	OsVersion     string `json:"os_version"`
	Vendor        string `json:"vendor"`
}

type DeviceMapper interface {
	AddNewDevice(device *Device) error
	RemoveDevice(device *Device) error
	GetDeviceByToken(token string) (*Device, error)
	GetDeviceById(device_id string) (*Device, error)
	GetDevicesById(device_ids []string) ([]*Device, error)
	GetAllDevice() ([]*Device, error)
}
