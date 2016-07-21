package fcm

import (
	"fcm/devicemapper"
	"fcm/env"
	"fmt"
	"fcm/gcm"
	"github.com/satori/go.uuid"
	"math/rand"
	"strconv"
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
	Tpl     string
	NewsId  string
	Title   string
	Digest  string
	Image   string
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
		env.Logger.Info("Token %s unregistered", msg.From)
		if device, err := env.DeviceMapper.GetDeviceByToken(msg.From); err == nil {
			if device != nil {
				env.DeviceMapper.RemoveDevice(device)
			}
		}
	}
}

func (appServer *AppServer) onReceipt(msg *gcm.CcsMessage) {
	env.Logger.Debug("OnReceipt from %v", msg.From)
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
				env.Logger.Info("device %v refreshed", device.DeviceId)
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
			env.Logger.Info("add new device %v to map", device.DeviceId)
			env.DeviceMapper.AddNewDevice(device)
			appServer.ConfirmRegistration(device, msg.MessageId)
		}

	case UPSTREAM_UNREGISTER:
		device_id, ok := msg.Data["device_id"].(string)
		if !ok {
			env.Logger.Info("error get device_id")
			return
		}
		if device, err := env.DeviceMapper.GetDeviceById(device_id); err != nil {
			env.Logger.Warn("get device error: %v", err)
			return
		} else {
			if device != nil {
				env.DeviceMapper.RemoveDevice(device)
			}
			env.Logger.Info("unregister from unseed device %v", device_id)

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
	env.Logger.Info("confirm register of %v:%v", device.DeviceId, device.Token)
}
func (appServer *AppServer) PushNotificationToDevice(dev *devicemapper.Device, notification *Notification) error {
	msg := getXmppMessageFromNotification(notification)
	msg.To = dev.Token

	env.Logger.Info("push message to %v:%v", dev.DeviceId, dev.Token)

	if _, _, err := gcm.SendXmpp(appServer.SenderId, appServer.SecurityKey, *msg); err != nil {
		env.Logger.Warn("send xmpp error: %v", err)
		return fmt.Errorf("send xmpp error: %v", err)
	}

	return nil
}

func (appServer *AppServer) BroadcastNotificationToTopic(topic string) error {
	msg_id := genMessageId()

	msg := &gcm.XmppMessage{
		MessageId:      msg_id,
		Priority:       HIGH_PRIORITY,
		DelayWhileIdle: false,
		TimeToLive:     600,
		Data: gcm.Data{
			"type": REREGISTER_TYPE,
		},
	}

	msg.To = fmt.Sprintf("/topics/%s", topic)

	if _, _, err := gcm.SendXmpp(appServer.SenderId, appServer.SecurityKey, *msg); err != nil {
		env.Logger.Warn("send xmpp error: %v", err)
		return fmt.Errorf("send xmpp error: %v", err)
	}

	return nil
}

func getXmppMessageFromNotification(notification *Notification) *gcm.XmppMessage {
	msg_id := genMessageId()
	push_id := strconv.Itoa(rand.Int() % 10000)

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
			"push_id": push_id,
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
