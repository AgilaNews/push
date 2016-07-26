package main

import (
	"encoding/json"
	"fcm/env"
	"fcm/fcm"
	"fcm/gcm"
	"fmt"
	"net/http"
	"os"
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

func home(w http.ResponseWriter, r *http.Request) {

}

func http_sendall(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Batch        int              `json:"batch"`
		Notification fcm.Notification `json:"notification"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("%v", err),
		})
		return
	}

	msg := &gcm.HttpMessage{
		Priority:       "high",
		DelayWhileIdle: &false_addr,
		TimeToLive:     new(int), //zero
		Data: gcm.Data{
			"type":    "1",
			"push_id": req.Notification.PushId,
			"tpl":     req.Notification.Tpl,
			"title":   req.Notification.Title,
			"digest":  req.Notification.Digest,
			"img":     req.Notification.Image,
			"news_id": req.Notification.NewsId,
		},
	}

	go func() {
		devices, err := env.DeviceMapper.GetAllDevice()
		env.Logger.Info("[SNDALL][%v]", len(devices))
		if err == nil {
			for i := 0; i < len(devices); i += req.Batch {
				msg.RegistrationIds = []string{}
				for _, device := range devices[i:min(i+req.Batch, len(devices))] {
					fmt.Println(device.Token)
					msg.RegistrationIds = append(msg.RegistrationIds, device.Token)
				}

				resp, err := gcm.SendHttp(env.Config.AppServer.SecurityKey, *msg)
				if err != nil {
					env.Logger.Info("[HTTP_BR_ERR][%v]", err)
				} else {
					env.Logger.Info("[HTTP_BR_RESP][%v]", resp)
				}
			}
		}
	}()

	json.NewEncoder(w).Encode(map[string]string{
		"error": "ok",
	})
}

func http_broadcast(w http.ResponseWriter, r *http.Request) {
	notification := &fcm.Notification{}
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("%v", err),
		})
		return
	}
	resp, err := gcm.SendHttp(env.Config.AppServer.SecurityKey, gcm.HttpMessage{
		To:             "/topics/" + BROADCASE_TOPIC,
		Priority:       "high",
		DelayWhileIdle: &false_addr,
		TimeToLive:     new(int), //zero
		Data: gcm.Data{
			"type":    "1",
			"push_id": notification.PushId,
			"tpl":     notification.Tpl,
			"title":   notification.Title,
			"digest":  notification.Digest,
			"img":     notification.Image,
			"news_id": notification.NewsId,
		},
	})
	env.Logger.Info("[HTTP_BROADCAST]")

	if err != nil {
		env.Logger.Info("[HTTP_BR_ERR][%v]", err)
	} else {
		env.Logger.Info("[HTTP_BR_RESP][%v]", resp)
	}
}

func http_sendToDeivces(w http.ResponseWriter, r *http.Request) {
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

	if device, err := env.DeviceMapper.GetDeviceById(req.To); err != nil || device == nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "device not found",
		})
		return
	} else {
		go func() {
			env.Logger.Info("[HTTP_SINGLECAST][%v]", device.DeviceId)
			resp, err := gcm.SendHttp(env.Config.AppServer.SecurityKey, gcm.HttpMessage{
				To:             device.Token,
				Priority:       "high",
				DelayWhileIdle: &false_addr,
				TimeToLive:     new(int),
				Data: gcm.Data{
					"type":    "1",
					"push_id": req.Notification.PushId,
					"tpl":     req.Notification.Tpl,
					"title":   req.Notification.Title,
					"digest":  req.Notification.Digest,
					"img":     req.Notification.Image,
					"news_id": req.Notification.NewsId,
				},
			})

			if err != nil {
				env.Logger.Info("[HTTP_BR_ERR][%v]", err)
			} else {
				env.Logger.Info("[HTTP_BR_RESP][%v]", resp)
			}
		}()
	}

	json.NewEncoder(w).Encode(map[string]string{
		"error": "ok",
	})
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
		env.Logger.Info("[SNDALL][%v]", len(devices))
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
	env.Logger.Info("[BROADCAST]")
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
	env.Logger.Info("[RESET]")
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
			env.Logger.Info("[SINGLECAST][%v]", device.DeviceId)
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

	http.HandleFunc("/http_broadcast", http_broadcast)
	http.HandleFunc("/http_send_to_deviceid", http_sendToDeivces)
	http.HandleFunc("/http_send_all_device", http_sendall)

	if err = http.ListenAndServe(env.Config.HttpServer.Addr, nil); err != nil {
		env.Logger.Error("listen on http server error")
	}
}
