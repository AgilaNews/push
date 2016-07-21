package main

import (
	"encoding/json"
	"fcm/env"
	"fcm/fcm"
	"fmt"
	"net/http"
	"os"
)

const (
	BROADCASE_TOPIC = "com.upeninsula.banews"
)

var (
	appServer *fcm.AppServer
)

func listClients(w http.ResponseWriter, r *http.Request) {
	devices, _ := env.DeviceMapper.GetAllDevice()

	json.NewEncoder(w).Encode(devices)
}

func home(w http.ResponseWriter, r *http.Request) {

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
	go func() {
		appServer.BroadcastReset(BROADCASE_TOPIC)
	}()
	json.NewEncoder(w).Encode(map[string]string{
		"error": "ok",
	})
}

func sendToDevices(w http.ResponseWriter, r *http.Request) {

}

func getNewsDetail(w http.ResponseWriter, r *http.Request) {

}

func main() {
	var err error
	if err = env.Init(); err != nil {
		fmt.Printf("init error: %v\n", err)
		os.Exit(-1)
	}

	appServer, err = fcm.NewAppServer(env.Config.AppServer.SenderId, env.Config.AppServer.SecurityKey)
	if err != nil {
		env.Logger.Error("init app server error")
		os.Exit(-1)
	} else {
		go appServer.Work()
	}

	http.HandleFunc("/clients", listClients)
	http.HandleFunc("/", home)
	http.HandleFunc("/broadcast", broadcast)
	http.HandleFunc("/send_all_device", sendall)
	http.HandleFunc("/reset", reset)
	http.HandleFunc("/send_to_deviceid", sendToDevices)
	http.HandleFunc("/news", getNewsDetail)

	if err = http.ListenAndServe(env.Config.HttpServer.Addr, nil); err != nil {
		env.Logger.Error("listen on http server error")
	}
}
