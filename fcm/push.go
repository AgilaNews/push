package fcm

import (
	"time"

	"database/sql"
	"encoding/json"
	"github.com/mcuadros/go-version"
	"push/task"
	"strconv"
	"sync"

	"github.com/alecthomas/log4go"
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

type Notification struct {
	Tpl     string               `json:"tpl"`
	NewsId  string               `json:"news_id"`
	Title   string               `json:"title"`
	Digest  string               `json:"digest,omitempty"`
	Image   string               `json:"image"`
	Options *NotificationOptions `json:"options,omitempty"`
	PushId  int                  `json:"push_id"`

	// internal use
	Condition struct {
		MinVersion    string `json:"min_version"`
		OS            string `json:"os"`
		LocalTimeZone string `json:"local_time_zone"`
	} `json:"-"`

	PlanTime    time.Time `json:"-"`
	DeliverTime time.Time `json:"-"`
	PushType    int       `json:"-"`
	DeliverType int       `json:"-"`
}

type NotificationOptions struct {
	Priority         string              `json:"priority"`
	DelayWhileIdle   *bool               `json:"delay_while_idle"`
	TTL              *int                `json:"ttl"`
	OnReceiptHandler *func(token string) //only take effect when you send to certain device
}

type PushManager struct {
	wdb *sql.DB
	rdb *sql.DB

	PushMap struct {
		sync.Mutex
		inner map[string]*Notification
	}
}

var (
	GlobalPushManager *PushManager
)

func (pushManager *PushManager) PushTaskHandler(push_id_str string, context interface{}) error {
	notification := context.(*Notification)

	if notification.PushType == PUSH_ALL {
		GlobalAppServer.BroadcastNotificationToTopic(BROAD_BROADCAST, notification)
	}

	return nil
}

func NewPushManager(taskManager *task.TaskManager, wdb, rdb *sql.DB) (*PushManager, error) {
	manager := &PushManager{
		wdb: wdb,
		rdb: rdb,

		PushMap: struct {
			sync.Mutex
			inner map[string]*Notification
		}{
			inner: make(map[string]*Notification),
		},
	}

	return manager, nil
}

func (p *PushManager) AddNotificationTask(at time.Time, push_type int, notification *Notification) error {
	notification.PlanTime = at
	notification.PushType = push_type

	if err := p.saveNotificationToDB(notification); err != nil {
		return err
	}

	t := task.NewOneshotTask(at, strconv.Itoa(notification.PushId), task.TASK_SOURCE_PUSH, 0, 0, p.PushTaskHandler, notification)
	if err := task.GlobalTaskManager.AddTask(t); err != nil {
		return err
	}

	return nil
}

func (pushManager *PushManager) saveNotificationToDB(notification *Notification) error {
	so, _ := json.Marshal(notification.Options)
	co, _ := json.Marshal(notification.Condition)
	now := int(time.Now().Unix())

	result, err := pushManager.wdb.Exec("INSERT INTO tb_push(`plan_time`, `deliver_time`, `deliver_type`, `push_condition`, `tpl`, "+
		"`title`, `digest`, `news_id`, `image`, `options`, `status`, `create_time`, `update_time`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		int(notification.PlanTime.Unix()),
		int(notification.DeliverTime.Unix()),
		notification.DeliverType,
		co,
		notification.Tpl,
		notification.Title,
		notification.Digest,
		notification.NewsId,
		notification.Image,
		string(so),
		STATUS_INIT,
		now,
		now,
	)

	if err != nil {
		return err
	}

	lid, _ := result.LastInsertId()
	notification.PushId = int(lid)
	log4go.Info("saved pushid is %d from %d", notification.PushId, lid)

	return nil
}

func (pushManager *PushManager) getAllVersion() ([]*ClientVersion, error) {
	rows, err := pushManager.rdb.Query("SELECT client_version FROM tb_version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cv := make([]string, 0)

	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cv = append(cv, c)
	}

	version.Sort(cv)

	ret := make([]*ClientVersion, len(cv))
	for idx, v := range cv {
		c := ClientVersion(v)
		ret[idx] = &c
	}
	return ret, nil
}
