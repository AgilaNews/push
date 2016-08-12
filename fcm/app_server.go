package fcm

import (
	"fmt"
	"push/device"
	"push/gcm"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/satori/go.uuid"
)

const (
	HIGH_PRIORITY     = "high"
	NORMAL_PRIORITY   = "normal"
	NOTIFICATION_TYPE = "1"
	CONFIRM_TYPE      = "2"
	REREGISTER_TYPE   = "3"

	TPL_IMAGE_WITH_TEXT = "2"

	REGISTER_SUCCESS = "0"
	REGISTER_FAILURE = "-1"

	UPSTREAM_REGISTER   = "1"
	UPSTREAM_UNREGISTER = "2"
)

var (
	true_addr   = true
	false_addr  = false
	default_ttl = 14400

	GlobalAppServer *AppServer
)

type AppServer struct {
	stop        chan bool
	SenderId    string
	SecurityKey string
	messageMap  map[string]*Notification

	client *gcm.XmppGcmClient
}

func (appServer *AppServer) onAck(msg *gcm.CcsMessage) {
	log4go.Global.Info("[OnAck][%s]", msg.MessageId)
}

func (appServer *AppServer) onNAck(msg *gcm.CcsMessage) {
	log4go.Global.Info("onNAck %v:%v", msg.MessageId, msg.Error)
	if msg.Error == "DEVICE_UNREGISTERED" {
		if d, err := device.GlobalDeviceMapper.GetDeviceByToken(msg.From); err == nil {
			if d != nil {
				log4go.Global.Info("[UNREG][%v]", d.DeviceId)
				device.GlobalDeviceMapper.RemoveDevice(d)
			}
		}
	}
}

func (appServer *AppServer) onReceipt(msg *gcm.CcsMessage) {
	if msg.Data["message_status"] == "MESSAGE_SENT_TO_DEVICE" {
		t, _ := strconv.ParseInt(msg.Data["message_sent_timestamp"].(string), 10, 64)
		at := time.Unix(t/1000, (t%1000)*1000).String()
		if d, err := device.GlobalDeviceMapper.GetDeviceByToken(msg.Data["device_registration_id"].(string)); err == nil {
			if d != nil {
				log4go.Global.Info("[RECEIVED][%v][%s][AT:%s]", d.DeviceId, msg.MessageId, at)
			} else {
				log4go.Global.Info("[RECEIVED][UNSEEN][%s][at%s]", msg.MessageId, at)
			}
		}
	} else {
		log4go.Global.Warn("[UNKNOWN_RECEPIT][%v]", msg)
	}

}

func (appServer *AppServer) onSendError(msg *gcm.CcsMessage) {
	log4go.Global.Debug("on send error %v", msg)
}

func (appServer *AppServer) onMessageReceived(msg *gcm.CcsMessage) {
	t := msg.Data["type"].(string)
	log4go.Global.Debug("on received message type:%v %v", t, msg)

	switch t {
	case UPSTREAM_REGISTER:
		token := getOrDefault(msg.Data, "token", "")
		device_id := getOrDefault(msg.Data, "device_id", "")
		client_version := getOrDefault(msg.Data, "client_version", "")
		imsi := getOrDefault(msg.Data, "imsi", "")
		os := getOrDefault(msg.Data, "os", "")
		os_version := getOrDefault(msg.Data, "os_version", "")
		vendor := getOrDefault(msg.Data, "vendor", "")

		if strings.HasPrefix(client_version, "v") {
			client_version = client_version[1:]
		}

		if d, err := device.GlobalDeviceMapper.GetDeviceById(device_id); err != nil {
			log4go.Global.Warn("get device error: %v", err)
			return
		} else {
			if d != nil && d.Token != token {
				log4go.Global.Info("[REM][%v]", d.DeviceId)
				device.GlobalDeviceMapper.RemoveDevice(d)
			}

			d = &device.Device{
				Token:         token,
				DeviceId:      device_id,
				ClientVersion: client_version,
				Imsi:          imsi,
				Os:            os,
				OsVersion:     os_version,
				Vendor:        vendor,
			}
			log4go.Global.Info("[NEW][%v]", d.DeviceId)
			device.GlobalDeviceMapper.AddNewDevice(d)
			appServer.ConfirmRegistration(d, msg.MessageId)
		}

	case UPSTREAM_UNREGISTER:
		device_id, ok := msg.Data["device_id"].(string)
		if !ok {
			return
		}
		if d, err := device.GlobalDeviceMapper.GetDeviceById(device_id); err != nil {
			log4go.Global.Warn("get device error: %v", err)
			return
		} else {
			if d != nil {
				device.GlobalDeviceMapper.RemoveDevice(d)
			}
			log4go.Global.Info("[UNREG_UNSEEN][%v]", device_id)

		}
	default:
		log4go.Global.Warn("unknown type %v", t)
	}
}

func NewAppServer(sender_id, sk string) (*AppServer, error) {
	gc, err := gcm.NewXmppGcmClient(sender_id, sk)
	if err != nil {
		return nil, err
	}

	appServer := &AppServer{
		stop:        make(chan bool),
		SenderId:    sender_id,
		SecurityKey: sk,
		client:      gc,
	}
	return appServer, nil
}

func (appServer *AppServer) Stop() {
	appServer.client.Close()
	appServer.stop <- true
}

func (appServer *AppServer) Work() {
	log4go.Global.Info("app server starts")

OUTFOR:
	for {
		if err := appServer.client.Listen(gcm.MessageHandler{
			OnAck:       appServer.onAck,
			OnNAck:      appServer.onNAck,
			OnMessage:   appServer.onMessageReceived,
			OnReceipt:   appServer.onReceipt,
			OnSendError: appServer.onSendError,
		}); err != nil {
			log4go.Global.Warn("listen to gcm error: %v", err)
		}

		select {
		case <-appServer.stop:
			log4go.Global.Info("appserver exits")
			break OUTFOR
		default:
		}

		log4go.Global.Info("reconnect")
	}

}

func NewNotificationDefaultOptions() *NotificationOptions {
	return &NotificationOptions{
		Priority:         HIGH_PRIORITY,
		DelayWhileIdle:   &false_addr,
		TTL:              &default_ttl,
		OnReceiptHandler: nil,
	}
}

func (appServer *AppServer) ConfirmRegistration(device *device.Device, msg_id string) {
	confirm := &gcm.XmppMessage{
		To:         device.Token,
		MessageId:  msg_id,
		Priority:   gcm.HighPriority,
		TimeToLive: &default_ttl,
		Data: gcm.Data{
			"type":   CONFIRM_TYPE,
			"status": REGISTER_SUCCESS,
		},
	}

	go appServer.client.Send(*confirm)
	log4go.Global.Info("[CONFIRMED][%v]", device.DeviceId)
}

func (appServer *AppServer) PushNotificationToDevice(dev *device.Device, notification *Notification) error {
	msg := getXmppMessageFromNotification(notification)
	msg.To = dev.Token

	t := dev.Token
	if len(t) > 32 {
		t = t[:32]
	}
	log4go.Global.Info("[NOTIFY][%v][%s]", dev.DeviceId, t)

	go appServer.client.Send(*msg)
	return nil
}

func (appServer *AppServer) BroadcastReset(topic string) error {
	msg_id := genMessageId()
	msg := &gcm.XmppMessage{
		To:             fmt.Sprintf("/topics/%s", topic),
		MessageId:      msg_id,
		Priority:       HIGH_PRIORITY,
		DelayWhileIdle: &false_addr,
		TimeToLive:     &default_ttl,
		Data: gcm.Data{
			"type": REREGISTER_TYPE,
		},
	}

	go appServer.client.Send(*msg)
	return nil
}

func (appServer *AppServer) BroadcastNotificationToMutliTopic(condition string, notification *Notification) {
	msg := getXmppMessageFromNotification(notification)
	msg.Condition = condition

	go appServer.client.Send(*msg)
}

func (appServer *AppServer) BroadcastNotificationToTopic(topic string, notification *Notification) {
	msg := getXmppMessageFromNotification(notification)
	msg.To = fmt.Sprintf("/topics/%s", topic)

	go appServer.client.Send(*msg)
}

func getXmppMessageFromNotification(notification *Notification) *gcm.XmppMessage {
	msg_id := genMessageId()

	return &gcm.XmppMessage{
		MessageId:                msg_id,
		Priority:                 notification.Options.Priority,
		DelayWhileIdle:           notification.Options.DelayWhileIdle,
		TimeToLive:               notification.Options.TTL,
		DeliveryReceiptRequested: &true_addr,
		ContentAvailable:         &true_addr,
		Data: gcm.Data{
			"type":    NOTIFICATION_TYPE,
			"push_id": notification.PushId,
			"tpl":     notification.Tpl,
			"title":   notification.Title,
			"digest":  notification.Digest,
			"img":     notification.Image,
			"news_id": notification.NewsId,
		},
	}
}

func genMessageId() string {
	return uuid.NewV4().String()
}

func getOrDefault(m gcm.Data, key, def string) string {
	if value, ok := m[key]; !ok {
		return def
	} else {
		return value.(string)
	}
}
