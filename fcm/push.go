package fcm

import (
	"time"

	"encoding/json"
	//	"github.com/mcuadros/go-version"
	"push/task"
	"strconv"

	"github.com/alecthomas/log4go"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

const (
	PUSH_ALL       = 1
	PUSH_TO_DEVICE = 2
	PUSH_TO_TOPIC  = 3

	STATUS_INIT       = 1
	STATUS_DOING      = 2
	STATUS_SCHEDULING = 3
	STATUS_DONE_FAIL  = 4
	STATUS_DONE_SUC   = 5
	STATUS_FINISH     = 6

	PUSH_MIN_VERSION = "v1.1.5"
	//	BROAD_BROADCAST  = "notification"
	BROAD_BROADCAST = "android_v1.1.7"
	//BROAD_BROADCAST = "com.upeninsula.banews"
)

type ClientVersion string

type PushContition struct {
	MinVersion    string `json:"min_version"`
	OS            string `json:"os"`
	LocalTimeZone string `json:"local_time_zone"`
}

type PushModel struct {
	gorm.Model

	Tpl       string               `gorm:"column:tpl;type:varchar(32)"`
	NewsId    string               `gorm:"column:news_id;type:varchar(32)";index`
	Title     string               `gorm:"column:title;type:varchar(256)"`
	Digest    string               `gorm:"column:digest;type:varchar(128)"`
	Image     string               `gorm:"column:image;type:varchar(1024);"`
	Options   *NotificationOptions `gorm:"-"`
	OptionStr string               `gorm:"column:option'type:varchar(1024);"`

	// internal use
	Condition    *PushContition `gorm:"-"`
	ConditionStr string         `gorm:"column:condition"`

	PlanTime    time.Time `gorm:"column:plan_time"`
	DeliverTime time.Time `gorm:"column:delivery_time"`
	DeliverType int       `gorm:"column:delivery_type;type:int(11)"`
	Status      int       `gorm:"column:status;type:tinyint(4)";index`
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
	OnReceiptHandler *func(token string) //only take effect when you send to certain device
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
		Status:  STATUS_INIT,
	}
}

func (pushManager *PushManager) PushTaskHandler(push_id_str string, context interface{}) error {
	push := context.(*PushModel)
	log4go.Info("handle push task at [%v] of push [%v]", time.Now(), push_id_str)

	if push.DeliverType == PUSH_ALL {
		notification := push.getNotification()
		GlobalAppServer.BroadcastNotificationToTopic(BROAD_BROADCAST, notification)
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

func (p *PushManager) AddPushTask(at time.Time, deliver_type int, condition *PushContition, notification *Notification) error {
	pushModel := notification.getPushModel()
	pushModel.PlanTime = at
	pushModel.DeliverType = deliver_type
	if condition != nil {
		tmp, _ := json.Marshal(condition)
		pushModel.ConditionStr = string(tmp)
	}

	if err := p.wdb.Create(&pushModel).Error; err != nil {
		return err
	}

	t := task.GlobalTaskManager.NewOneshotTask(at, strconv.Itoa(int(pushModel.ID)), task.TASK_SOURCE_PUSH, 0, 0, pushModel)
	if err := task.GlobalTaskManager.AddTask(t); err != nil {
		return err
	}

	return nil
}

func (p *PushManager) GetPush(id string) (*PushModel, error) {
	pushModel := &PushModel{}

	if err := p.rdb.First(pushModel, id).Error; err != nil {
		return pushModel, nil
	} else {
		return nil, err
	}
}

func (p *PushManager) BatchGetPush(ids []string) ([]*PushModel, error) {
	models := make([]*PushModel, 0)
	if err := p.rdb.Find(&models, ids).Error; err != nil {
		return nil, err
	} else {
		return models, nil
	}
}

func (p *PushManager) GetPushs(page_number, page_size int) ([]*PushModel, error) {
	models := make([]*PushModel, 0)

	off := page_number * page_size
	if err := p.rdb.Find(&models).Offset(off).Limit(page_size).Error; err != nil {
		return nil, err
	} else {
		return models, nil
	}
}
