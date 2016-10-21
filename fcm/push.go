package fcm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/AgilaNews/push/device"
	"github.com/AgilaNews/push/task"

	"github.com/alecthomas/log4go"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/mcuadros/go-version"
)

const (
	PUSH_ALL       = PushType(1)
	PUSH_MULTICAST = PushType(2)

	PUSH_MIN_VERSION = "1.1.5"

	PLATFORM_ALL     = "all"
	PLATFORM_ANDROID = "android"
	PLATFORM_IOS     = "ios"

	IOS_PUBLISHED     = 0x2
	ANDROID_PUBLISHED = 0x1
)

type PushType int

type ClientVersion struct {
	ID            string `gorm:"column:id;primary_key;" json:"-"`
	Version       string `gorm:"column:client_version;type:varchar(16);" json:"client_version"`
	ServerVersion string `gorm:"column:server_version;type:int(11);" json:"-"`
	Status        int    `gorm:"column:status;type:tinyint(3);" json:"-"`
}

type PushCondition struct {
	Devices    []string `json:"devices"`
	MinVersion string   `json:"min_version"`
	OS         string   `json:"os"`
	Platform   string   `json:"platform"`
	//	LocalTimeZone string   `json:"local_time_zone"`
}

type PushModel struct {
	ID             uint      `gorm:"column:id;primary_key" json:"push_id"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"-"`
	CreatedAtUnix  int64     `gorm:"-" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"-"`
	UpdatedAtUnix  int64     `gorm:"-" json:"updated_at"`
	CanceledAt     time.Time `gorm:"column:canceled_at" json:"-"`
	CanceledAtUnix int64     `gorm:"-" json:"canceled_at"`

	Tpl       string               `gorm:"column:tpl;type:varchar(32)" json:"tpl"`
	NewsId    string               `gorm:"column:news_id;type:varchar(32);index" json:"news_id"`
	Title     string               `gorm:"column:title;type:varchar(256)" json:"title"`
	Digest    string               `gorm:"column:digest;type:varchar(128)" json:"digest"`
	Image     string               `gorm:"column:image;type:varchar(1024);" json:"image"`
	Options   *NotificationOptions `gorm:"-" json:"options"`
	OptionStr string               `gorm:"column:option;type:varchar(4096);" json:"-"`

	// internal use
	ConditionStr string         `gorm:"column:condition;type:varchar(4096);" json:"-"`
	Condition    *PushCondition `gorm:"-" json:"condition"`

	PlanTime     time.Time `gorm:"column:plan_time" json:"-"`
	PlanTimeUnix int64     `gorm:"-" json:"plan_time"`
	DeliverType  PushType  `gorm:"column:delivery_type;type:int(11)" json:"deliver_type"`

	Click     int     `gorm:"column:click" json:"click"`
	Reach     int     `gorm:"column:reach" json:"reach"`
	ClickRate float32 `gorm:"column:click_rate" json:"click_rate"`

	//from task, we don't use foreign keys because it introducd unnesscary complex
	Status          task.TaskStatus `gorm:"-" json:"status"`
	DeliverTime     time.Time       `gorm:"deliver_time" json:"-"`
	DeliverTimeUnix int64           `grom:"-" json:"deliver_time"`
}

//this type is sent to FCM server
type Notification struct {
	Tpl     string               `json:"tpl"`
	NewsId  string               `json:"news_id"`
	Title   string               `json:"title"`
	Digest  string               `json:"digest,omitempty"`
	Image   string               `json:"image"`
	Options *NotificationOptions `json:"options,omitempty"`
	PushId  int                  `json:"push_id"`
}

type NotificationOptions struct {
	Priority       string `json:"priority"`
	DelayWhileIdle *bool  `json:"delay_while_idle"`
	TTL            *int   `json:"ttl"`
	//	ContentAvaliable *bool               `json:"content_avaliable"`
	OnReceiptHandler *func(token string) `json:"-"` //only take effect when you send to certain device
}

type PushManager struct {
	wdb *gorm.DB
	rdb *gorm.DB
}

var (
	GlobalPushManager *PushManager
)

func (PushModel) TableName() string {
	return "tb_push"
}

func (m *PushModel) BeforeCreate() error {
	var t []byte
	t, _ = json.Marshal(m.Options)
	m.OptionStr = string(t)
	t, _ = json.Marshal(m.Condition)
	m.ConditionStr = string(t)

	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	m.CanceledAt = time.Unix(0, 0)
	m.DeliverTime = time.Unix(0, 0)

	m.CreatedAtUnix = m.CreatedAt.Unix()
	m.UpdatedAtUnix = m.UpdatedAt.Unix()
	m.DeliverTimeUnix = m.DeliverTime.Unix()

	m.PlanTimeUnix = m.PlanTime.Unix()
	m.CanceledAtUnix = m.CanceledAt.Unix()
	return nil
}

func (m *PushModel) BeforeUpdate() error {
	var t []byte
	t, _ = json.Marshal(m.Options)
	m.OptionStr = string(t)
	t, _ = json.Marshal(m.Condition)
	m.ConditionStr = string(t)

	m.UpdatedAt = time.Now()

	m.CreatedAtUnix = m.CreatedAt.Unix()
	m.UpdatedAtUnix = m.UpdatedAt.Unix()
	m.DeliverTimeUnix = m.DeliverTime.Unix()
	m.PlanTimeUnix = m.PlanTime.Unix()
	m.CanceledAtUnix = m.CanceledAt.Unix()
	return nil
}

func (m *PushModel) AfterFind() error {
	if len(m.OptionStr) > 0 {
		m.Options = &NotificationOptions{}

		if err := json.Unmarshal([]byte(m.OptionStr), m.Options); err != nil {
			return err
		}
	}

	if len(m.ConditionStr) > 0 {
		m.Condition = &PushCondition{}

		if err := json.Unmarshal([]byte(m.ConditionStr), m.Condition); err != nil {
			return err
		}
	}

	m.CreatedAtUnix = m.CreatedAt.Unix()
	m.UpdatedAtUnix = m.UpdatedAt.Unix()
	m.CanceledAtUnix = m.CanceledAt.Unix()
	m.PlanTimeUnix = m.PlanTime.Unix()
	m.DeliverTimeUnix = m.DeliverTime.Unix()

	return nil
}

func (ClientVersion) TableName() string {
	return "tb_version"
}

func (p *PushModel) getNotification() *Notification {
	return &Notification{
		PushId:  int(p.ID),
		Tpl:     p.Tpl,
		NewsId:  p.NewsId,
		Title:   p.Title,
		Digest:  p.Digest,
		Image:   p.Image,
		Options: p.Options,
	}
}

func (n *Notification) getPushModel() *PushModel {
	return &PushModel{
		Tpl:     n.Tpl,
		NewsId:  n.NewsId,
		Title:   n.Title,
		Digest:  n.Digest,
		Image:   n.Image,
		Options: n.Options,
	}
}

func (pushManager *PushManager) DoTask(push_id_str string, context interface{}) error {
	push := context.(*PushModel)
	log4go.Info("handle push task at [%v] of push [%v]", time.Now(), push_id_str)

	notification := push.getNotification()

	switch push.DeliverType {
	case PUSH_ALL:
		if android_topics, ios_topics, err := pushManager.getTopics(push.Condition); err != nil {
			log4go.Warn("get topic error :%v", err)
		} else {
			GlobalAppServer.BroadcastNotifications(ios_topics, notification)
			GlobalAppServer.BroadcastNotifications(android_topics, notification)
		}
	case PUSH_MULTICAST:
		notification.PushId = 1
		///check push model validation
		if err := pushManager.InstantMulticast(push.Condition.Devices, notification); err != nil {
			log4go.Warn("instant push error : %v", err)
		}
	}

	return nil
}

func NewPushManager(taskManager *task.TaskManager, wdb, rdb *gorm.DB) (*PushManager, error) {
	manager := &PushManager{
		wdb: wdb,
		rdb: rdb,
	}

	return manager, nil
}

func (p *PushManager) InstantMulticast(device_ids []string, notification *Notification) error {
	notification.PushId = 1

	if devices, err := device.GlobalDeviceMapper.GetDevicesById(device_ids); err != nil {
		return err
	} else {
		for _, device := range devices {
			if device != nil {
				GlobalAppServer.PushNotificationToDevice(device, notification)
			}
		}
	}
	return nil
}

func (p *PushManager) NewPushMessage(at time.Time, deliver_type PushType, condition *PushCondition, notification *Notification) (*PushModel, error) {
	pushModel := notification.getPushModel()
	pushModel.PlanTime = at
	pushModel.DeliverType = deliver_type
	pushModel.Condition = condition

	if err := p.wdb.Create(&pushModel).Error; err != nil {
		return nil, err
	}

	return pushModel, nil
}

func (p *PushManager) UpdatePush(push_id int, at time.Time, condition *PushCondition, n *Notification) (*PushModel, error) {
	var pushModel PushModel
	if err := p.rdb.First(&pushModel, push_id).Error; err != nil {
		return nil, err
	}

	pushModel.PlanTime = at
	pushModel.Condition = condition
	pushModel.Tpl = n.Tpl
	pushModel.NewsId = n.NewsId
	pushModel.Title = n.Title
	pushModel.Digest = n.Digest
	pushModel.Image = n.Image
	pushModel.Options = n.Options

	if err := p.rdb.Save(pushModel).Error; err != nil {
		return nil, err
	}

	return &pushModel, nil
}

func (p *PushManager) FirePushTask(id uint) error {
	pushModel := &PushModel{}
	if err := p.rdb.Where(id).First(pushModel).Error; err != nil {
		return err
	}

	if err := p.getTaskStatusOfPushes([]*PushModel{pushModel}); err != nil {
		return err
	}

	if pushModel.Status != task.STATUS_INIT {
		return fmt.Errorf("only task in edit status could be fired")
	}

	t := task.GlobalTaskManager.NewOneshotTask(pushModel.PlanTime,
		strconv.Itoa(int(pushModel.ID)),
		task.TASK_SOURCE_PUSH,
		0,
		0,
		pushModel)

	if err := task.GlobalTaskManager.AddAndScheduleTask(t); err != nil {
		return err
	}

	return nil
}

func (p *PushManager) CancelPush(id uint) error {
	pushModel := &PushModel{}
	if err := p.rdb.Where(id).First(pushModel).Error; err != nil {
		return err
	}

	if err := p.getTaskStatusOfPushes([]*PushModel{pushModel}); err != nil {
		return err
	}

	if pushModel.Status != task.STATUS_PENDING {
		return fmt.Errorf("only task in scheduling could be canceld, running task is too late to stop")
	}

	uid := strconv.Itoa(int(pushModel.ID))
	if err := task.GlobalTaskManager.CancelTask(uid, task.TASK_SOURCE_PUSH); err != nil {
		return err
	}

	pushModel.CanceledAt = time.Now()

	if err := p.wdb.Save(pushModel).Error; err != nil {
		return err
	}

	return nil
}

func (p *PushManager) Sync(uid string) (interface{}, error) {
	pushModel := &PushModel{}

	if err := p.rdb.First(pushModel, uid).Error; err != nil {
		return nil, err
	} else {
		var v NotificationOptions

		if err := json.Unmarshal([]byte(pushModel.OptionStr), &v); err != nil {
			return nil, err
		} else {
			pushModel.Options = &v
		}

		return pushModel, nil
	}
}

func (p *PushManager) GetPush(id int) (*PushModel, error) {
	pushModel := &PushModel{}

	if err := p.rdb.First(pushModel, id).Error; err != nil {
		return nil, err
	} else {
		if err = p.getTaskStatusOfPushes([]*PushModel{pushModel}); err != nil {
			return nil, err
		} else {
			return pushModel, nil
		}
	}
}

func (p *PushManager) BatchGetPush(ids []string) ([]*PushModel, error) {
	models := make([]*PushModel, 0)
	if err := p.rdb.Find(&models, ids).Error; err != nil {
		return nil, err
	} else {
		if err = p.getTaskStatusOfPushes(models); err != nil {
			return nil, err
		} else {
			return models, nil
		}
	}
}

func (p *PushManager) GetPushs(start, length int, filters map[string]string) ([]*PushModel, int, error) {
	models := make([]*PushModel, 0)
	var count int

	if err := p.rdb.Find(&models).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var db *gorm.DB = p.rdb
	if len(filters) != 0 {
		db = db.Where(filters)
	}

	if err := p.rdb.Offset(start).Limit(length).Order("id desc").Find(&models).Error; err != nil {
		return nil, 0, err
	} else {
		if err = p.getTaskStatusOfPushes(models); err != nil {
			return nil, 0, err
		} else {
			return models, count, nil
		}
	}
}

func (p *PushManager) getTaskStatusOfPushes(models []*PushModel) error {
	uids := make([]string, len(models))
	m := make(map[string]*PushModel)

	var tasks []*task.Task

	for idx, push := range models {
		uid := strconv.Itoa(int(push.ID))
		uids[idx] = uid
		m[uid] = push
	}

	if err := p.rdb.Where("uid in (?)", uids).Find(&tasks).Error; err != nil {
		return err
	} else {
		for _, t := range tasks {
			log4go.Debug("task %v", t)
			if _, ok := m[t.UserIdentifier]; ok {
				m[t.UserIdentifier].Status = t.Status
				m[t.UserIdentifier].DeliverTime = t.LastExecutionTime
				m[t.UserIdentifier].DeliverTimeUnix = t.LastExecutionTime.Unix()
				delete(m, t.UserIdentifier)
			}
		}

		for _, pushModel := range m {
			pushModel.Status = task.STATUS_INIT
		}
	}

	return nil
}

func (p *PushManager) getTopics(condition *PushCondition) ([]string, []string, error) {
	log4go.Debug("get topics of condition %v", condition)
	var versions []ClientVersion
	if err := p.rdb.Find(&versions).Error; err != nil {
		return nil, nil, err
	}

	var android_topics, ios_topics []string
	var minV string

	if nil == condition || len(condition.MinVersion) == 0 {
		minV = version.Normalize(PUSH_MIN_VERSION)
	} else {
		minV = condition.MinVersion
	}

	for _, versionModel := range versions {
		if version.Compare(versionModel.Version, minV, ">=") {
			if (condition.Platform == PLATFORM_ALL || condition.Platform == PLATFORM_IOS) && (versionModel.Status&IOS_PUBLISHED != 0) {
				log4go.Debug("version %v, status %v, added to ios", versionModel.Version, versionModel.Status)
				ios_topics = append(ios_topics, fmt.Sprintf("ios_v%s", versionModel.Version))
			}

			if (condition.Platform == PLATFORM_ALL || condition.Platform == PLATFORM_ANDROID) && (versionModel.Status&ANDROID_PUBLISHED != 0) {
				log4go.Debug("version %v, status %v, added to android", versionModel.Version, versionModel.Status)
				android_topics = append(android_topics, fmt.Sprintf("android_v%s", versionModel.Version))
			}
		}
	}

	return android_topics, ios_topics, nil
}
