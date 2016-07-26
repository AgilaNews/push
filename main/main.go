package main

import (
	"fcm/env"

	"encoding/json"
	"fcm/fcm"
	"fmt"
	"net/http"
	"os"

	"github.com/alecthomas/log4go"
)

const (
	BROADCASE_TOPIC = "com.upeninsula.banews"
)

var (
	appServer  *fcm.AppServer
	true_addr  = true
	false_addr = false
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func listClients(w http.ResponseWriter, r *http.Request) {
	devices, _ := env.DeviceMapper.GetAllDevice()

	json.NewEncoder(w).Encode(devices)
}

func sendall(w http.ResponseWriter, r *http.Request) {
	notification := &fcm.Notification{}
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("%v", err),
		})
		return
	}

	if notification.Tpl != fcm.TPL_IMAGE_WITH_TEXT {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "only support tpl 2",
		})
		return
	}

	notification.Options = fcm.NewNotificationDefaultOptions()
	go func() {
		devices, err := env.DeviceMapper.GetAllDevice()
		log4go.Global.Info("[SNDALL][%v]", len(devices))
		if err == nil {
			for _, device := range devices {
				appServer.PushNotificationToDevice(device, notification)
			}
		}
	}()
	json.NewEncoder(w).Encode(map[string]string{
		"error": "ok",
	})
}

func broadcast(w http.ResponseWriter, r *http.Request) {
	log4go.Global.Info("[BROADCAST]")
	notification := &fcm.Notification{}

	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("%v", err),
		})
		return
	}

	if notification.Tpl != fcm.TPL_IMAGE_WITH_TEXT {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "only support tpl 2",
		})
		return
	}

	notification.Options = fcm.NewNotificationDefaultOptions()
	go func() {
		appServer.BroadcastNotificationToTopic(BROADCASE_TOPIC, notification)
	}()

	json.NewEncoder(w).Encode(map[string]string{
		"error": "ok",
	})
}

func reset(w http.ResponseWriter, r *http.Request) {
	log4go.Global.Info("[RESET]")
	go func() {
		appServer.BroadcastReset(BROADCASE_TOPIC)
	}()
	json.NewEncoder(w).Encode(map[string]string{
		"error": "ok",
	})
}

func sendToDevices(w http.ResponseWriter, r *http.Request) {
	req := struct {
		To           string           `json:"to"`
		Notification fcm.Notification `json:"notification"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("%v", err),
		})
		return
	}

	if req.Notification.Tpl != fcm.TPL_IMAGE_WITH_TEXT {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "only support tpl 2",
		})
		return
	}
	req.Notification.Options = fcm.NewNotificationDefaultOptions()

	if device, err := env.DeviceMapper.GetDeviceById(req.To); err != nil || device == nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "device not found",
		})
		return
	} else {
		go func() {
			log4go.Global.Info("[SINGLECAST][%v]", device.DeviceId)
			appServer.PushNotificationToDevice(device, &req.Notification)
		}()
	}

	json.NewEncoder(w).Encode(map[string]string{
		"error": "ok",
	})
}

func getNewsDetail(w http.ResponseWriter, r *http.Request) {

}

func main() {
	fmt.Printf("starts")
	var err error

	if err = env.Init(); err != nil {
		fmt.Printf("init error: %v\n", err)
		os.Exit(-1)
	}

	appServer, err = fcm.NewAppServer(env.Config.AppServer.SenderId, env.Config.AppServer.SecurityKey)
	if err != nil {
		log4go.Global.Error("init app server error")
		os.Exit(-1)
	} else {
		go appServer.Work()
	}

	http.HandleFunc("/clients", listClients)
	http.HandleFunc("/broadcast", broadcast)
	http.HandleFunc("/send_all_device", sendall)
	http.HandleFunc("/reset", reset)
	http.HandleFunc("/send_to_deviceid", sendToDevices)
	http.HandleFunc("/news", getNewsDetail)

	if err = http.ListenAndServe(env.Config.HttpServer.Addr, nil); err != nil {
		log4go.Global.Error("listen on http server error")
	}
}
