package fcm

import (
	"encoding/json"
	"fmt"
	"push/device"
	"push/task"
	"strconv"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/mcuadros/go-version"
)

const (
	PUSH_ALL       = PushType(1)
	PUSH_MULTICAST = PushType(2)

	PUSH_MIN_VERSION = "v1.1.5"
	BROAD_BROADCAST  = "android_%v"
)

type PushType int

type ClientVersion struct {
	ID            string `gorm:"column:id;primary_key;" json:"-"`
	Version       string `gorm:"column:client_version;type:varchar(16);" json:"client_version"`
	ServerVersion string `gorm:"column:server_version;type:int(11);" json:"-"`
}

type PushCondition struct {
	Devices    []string `json:"devices"`
	MinVersion string   `json:"min_version"`
	OS         string   `json:"os"`
	//	LocalTimeZone string   `json:"local_time_zone"`
}

type PushModel struct {
	ID        uint      `gorm:"column:id;primary_key" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`

	To        string               `gorm:"column:to;type:varchar(1024)" json:"to"`
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

	PlanTime    time.Time `gorm:"column:plan_time" json:"plan_time"`
	DeliverType PushType  `gorm:"column:delivery_type;type:int(11)" json:"deliver_type"`

	Click     int     `gorm:"column:click" json:"click"`
	Reach     int     `gorm:"column:reach" json:"reach"`
	ClickRate float32 `gorm:"column:click_rate" json:"click_rate"`

	//from task, we don't use foreign keys because it introducd unnesscary complex
	Status      task.TaskStatus `gorm:"-" json:"status"`
	DeliverTime time.Time       `gorm:"-" json:"deliver_time"`
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
	Priority         string              `json:"priority"`
	DelayWhileIdle   *bool               `json:"delay_while_idle"`
	TTL              *int                `json:"ttl"`
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
	return nil
}

func (m *PushModel) BeforeUpdate() error {
	var t []byte
	t, _ = json.Marshal(m.Options)
	m.OptionStr = string(t)
	t, _ = json.Marshal(m.Condition)
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

	switch push.DeliverType {
	case PUSH_ALL:
		notification := push.getNotification()
		if topics, err := pushManager.getTopics(push.Condition); err != nil {
			log4go.Warn("get push error : %v", err)
		} else {
			for _, topic := range topics {
				GlobalAppServer.BroadcastNotificationToTopic(topic, notification)
				log4go.Info("send to topic [%v]", topic)
			}

		}

	case PUSH_MULTICAST:
		notification := push.getNotification()
		///check push model validation
		if devices, err := device.GlobalDeviceMapper.GetDevicesById(push.Condition.Devices); err != nil {
			return err
		} else {
			for _, device := range devices {
				GlobalAppServer.PushNotificationToDevice(device, notification)
			}
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

func (p *PushManager) NewPushMessage(at time.Time, deliver_type PushType, condition *PushCondition, notification *Notification) (*PushModel, error) {
	pushModel := notification.getPushModel()
	pushModel.PlanTime = at
	pushModel.DeliverType = deliver_type

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

func (p *PushManager) FirePushTask(pushModel *PushModel) error {
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
		return pushModel, nil
	}
}

func (p *PushManager) BatchGetPush(ids []string) ([]*PushModel, error) {
	models := make([]*PushModel, 0)
	if err := p.rdb.Find(&models, ids).Error; err != nil {
		return nil, err
	} else {
		if err = p.GetTaskStatusOfPushes(models); err != nil {
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
		if err = p.GetTaskStatusOfPushes(models); err != nil {
			return nil, 0, err
		} else {
			return models, count, nil
		}
	}
}

func (p *PushManager) GetTaskStatusOfPushes(models []*PushModel) error {
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
			if _, ok := m[t.UserIdentifier]; ok {
				m[t.UserIdentifier].Status = t.Status
				m[t.UserIdentifier].DeliverTime = t.LastExecutionTime
			}
		}
	}

	return nil
}

func (p *PushManager) getTopics(condition *PushCondition) ([]string, error) {
	var versions []ClientVersion
	if err := p.rdb.Find(&versions).Error; err != nil {
		return nil, err
	}

	topics := make([]string, 0)

	var minV string
	if nil == condition {
		minV = version.Normalize(PUSH_MIN_VERSION)
	} else {
		minV = condition.MinVersion
	}

	for _, versionModel := range versions {
		if version.Compare(versionModel.Version, minV, ">=") {
			topics = append(topics, fmt.Sprintf(BROAD_BROADCAST, versionModel.Version))
		}
	}

	return topics, nil
}
