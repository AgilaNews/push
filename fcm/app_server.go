package fcm

import (
	"fcm/devicemapper"
	"fcm/env"
	"fcm/gcm"
	"fmt"
	"github.com/satori/go.uuid"
	"strings"
	"time"
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

type AppServer struct {
	stop        chan bool
	SenderId    string
	SecurityKey string
	messageMap  map[string]*Notification
}

type Notification struct {
	Tpl     string `json:"tpl"`
	NewsId  string `json:"news_id"`
	Title   string `json:"title"`
	Digest  string `json:"digest,omitempty"`
	Image   string `json:"image"`
	PushId  int    `json:"push_id"`
	Options *NotificationOptions
}

type NotificationOptions struct {
	Priority         string
	DelayWhileIdle   bool
	TTL              int
	OnReceiptHandler *func(token string) //only take effect when you send to certain device
}

func (appServer *AppServer) onAck(msg *gcm.CcsMessage) {
	env.Logger.Debug("OnAck %v from %v received", msg.MessageId, msg.From)
}

func (appServer *AppServer) onNAck(msg *gcm.CcsMessage) {
	env.Logger.Debug("onNAck %v", msg)
	if msg.Error == "DEVICE_UNREGISTERED" {
		if device, err := env.DeviceMapper.GetDeviceByToken(msg.From); err == nil {
			if device != nil {
				env.Logger.Info("[UNREG][%v]", device.DeviceId)
				env.DeviceMapper.RemoveDevice(device)
			}
		}
	}
}

func (appServer *AppServer) onReceipt(msg *gcm.CcsMessage) {
	if msg.Data["message_status"] == "MESSAGE_SENT_TO_DEVICE" {
		env.Logger.Debug("OnReceipt from %v at %v", msg.Data["device_registration_id"], msg.Data["message_sent_timestamp"])
        env.Logger.Info("[RECEIVED][%v]", msg.Data["device_registration_id"])
	}
}

func (appServer *AppServer) onSendError(msg *gcm.CcsMessage) {
	env.Logger.Debug("on send error %v", msg)
}

func (appServer *AppServer) onMessageReceived(msg *gcm.CcsMessage) {
	t := msg.Data["type"].(string)
	env.Logger.Debug("on received message type:%v %v", t, msg)

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

		if device, err := env.DeviceMapper.GetDeviceById(device_id); err != nil {
			env.Logger.Warn("get device error: %v", err)
			return
		} else {
			if device != nil && device.Token != token {
				env.Logger.Info("[REF][%v]", device.DeviceId)
				env.DeviceMapper.RemoveDevice(device)
			}

			device = &devicemapper.Device{
				Token:         token,
				DeviceId:      device_id,
				ClientVersion: client_version,
				Imsi:          imsi,
				Os:            os,
				OsVersion:     os_version,
				Vendor:        vendor,
			}
			env.Logger.Info("[NEW][%v]", device.DeviceId)
			env.DeviceMapper.AddNewDevice(device)
			appServer.ConfirmRegistration(device, msg.MessageId)
		}

	case UPSTREAM_UNREGISTER:
		device_id, ok := msg.Data["device_id"].(string)
		if !ok {
			return
		}
		if device, err := env.DeviceMapper.GetDeviceById(device_id); err != nil {
			env.Logger.Warn("get device error: %v", err)
			return
		} else {
			if device != nil {
				env.DeviceMapper.RemoveDevice(device)
			}
			env.Logger.Info("[UNREG_UNSEEN][%v]", device_id)

		}
	default:
		env.Logger.Warn("unknown type %v", t)
	}
}

func NewAppServer(sender_id, sk string) (*AppServer, error) {
	appServer := &AppServer{
		stop:        make(chan bool),
		SenderId:    sender_id,
		SecurityKey: sk,
	}
	return appServer, nil
}

func (appServer *AppServer) Stop() {
	appServer.stop <- true
}

func (appServer *AppServer) Work() {
	env.Logger.Info("app server starts")
	gcm.DebugMode = true
	for {
		if err := gcm.Listen(appServer.SenderId, appServer.SecurityKey, gcm.MessageHandler{
			OnAck:       appServer.onAck,
			OnNAck:      appServer.onNAck,
			OnMessage:   appServer.onMessageReceived,
			OnReceipt:   appServer.onReceipt,
			OnSendError: appServer.onSendError,
		}, appServer.stop); err != nil {
			//if err == io.EOF {
			appServer.stop <- true
			time.Sleep(3 * time.Second)
			//}
			env.Logger.Error("listen to gcm error: %v", err)
		}
		time.Sleep(time.Second)
	}
}

func NewNotificationDefaultOptions() *NotificationOptions {
	return &NotificationOptions{
		Priority:         HIGH_PRIORITY,
		DelayWhileIdle:   true,
		TTL:              600,
		OnReceiptHandler: nil,
	}
}

func (appServer *AppServer) ConfirmRegistration(device *devicemapper.Device, msg_id string) {
	confirm := &gcm.XmppMessage{
		To:         device.Token,
		MessageId:  msg_id,
		Priority:   gcm.HighPriority,
		TimeToLive: 600,
		Data: gcm.Data{
			"type":   CONFIRM_TYPE,
			"status": REGISTER_SUCCESS,
		},
	}

	if _, _, err := gcm.SendXmpp(appServer.SenderId,
		appServer.SecurityKey, *confirm); err != nil {
		env.Logger.Warn("send confirm error %v", err)
	}
	env.Logger.Info("[CONFIRMED][%v]", device.DeviceId)
}
func (appServer *AppServer) PushNotificationToDevice(dev *devicemapper.Device, notification *Notification) error {
	msg := getXmppMessageFromNotification(notification)
	msg.To = dev.Token

	env.Logger.Info("[NOTIFY][%v]", dev.DeviceId)

	if _, _, err := gcm.SendXmpp(appServer.SenderId, appServer.SecurityKey, *msg); err != nil {
		env.Logger.Warn("send xmpp error: %v", err)
		return fmt.Errorf("send xmpp error: %v", err)
	}

	return nil
}

func (appServer *AppServer) BroadcastReset(topic string) error {
	msg_id := genMessageId()
	msg := &gcm.XmppMessage{
		To:             fmt.Sprintf("/topics/%s", topic),
		MessageId:      msg_id,
		Priority:       HIGH_PRIORITY,
		DelayWhileIdle: false,
		TimeToLive:     600,
		Data: gcm.Data{
			"type": REREGISTER_TYPE,
		},
	}

	if _, _, err := gcm.SendXmpp(appServer.SenderId, appServer.SecurityKey, *msg); err != nil {
		env.Logger.Warn("send xmpp error: %v", err)
		return fmt.Errorf("send xmpp error: %v", err)
	}

	return nil

}

func (appServer *AppServer) BroadcastNotificationToTopic(topic string, notification *Notification) error {
	msg := getXmppMessageFromNotification(notification)
	msg.To = fmt.Sprintf("/topics/%s", topic)

	if _, _, err := gcm.SendXmpp(appServer.SenderId, appServer.SecurityKey, *msg); err != nil {
		env.Logger.Warn("send xmpp error: %v", err)
		return fmt.Errorf("send xmpp error: %v", err)
	}

	return nil
}

func getXmppMessageFromNotification(notification *Notification) *gcm.XmppMessage {
	msg_id := genMessageId()

	return &gcm.XmppMessage{
		MessageId: msg_id,
		//        CollapseKey: "",
		Priority:                 notification.Options.Priority,
		DelayWhileIdle:           notification.Options.DelayWhileIdle,
		TimeToLive:               notification.Options.TTL,
		DeliveryReceiptRequested: true,
		// TODO handle confirm
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
