package main

import (
	"bufio"
	"encoding/json"
	"fcm/env"
	"fcm/fcm"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	SENDER_ID       = "759516127227"
	SECURITY_KEY    = "AIzaSyDVISzLzFTG8qgasug3BM_cDgw-mtkj0u0"
	BROADCASE_TOPIC = "com.upeninsula.banews"
	HTTP_BIND       = ":8070"
)

func ListClients(w http.ResponseWriter, r *http.Request) {
	devices, _ := env.DeviceMapper.GetAllDevice()

	json.NewEncoder(w).Encode(devices)
}

func main() {
	if err := env.Init(); err != nil {
		fmt.Printf("init error: %v\n", err)
		os.Exit(-1)
	}

	appServer, err := fcm.NewAppServer(SENDER_ID, SECURITY_KEY)
	if err != nil {
		env.Logger.Error("init app server error")
		os.Exit(-1)
	} else {
		go appServer.Work()
	}

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for true {
			text, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("exit")
				os.Exit(-1)
			}

			switch strings.TrimSpace(text) {
			case "broadcast":
				appServer.BroadcastNotificationToTopic(BROADCASE_TOPIC)
			case "sendall":
				notification := &fcm.Notification{
					Tpl:     fcm.TPL_IMAGE_WITH_TEXT,
					NewsId:  "4uVadtC/INM=",
					Title:   "e-Man and the Masters of the",
					Digest:  "Lian zhan is animal",
					Image:   "http://img.agilanews.info/image/MvMnVzjhMXU%3D.jpg?t=135x135",
					Options: fcm.NewNotificationDefaultOptions(),
				}
				devices, err := env.DeviceMapper.GetAllDevice()
				if err == nil {
					for _, device := range devices {
						appServer.PushNotificationToDevice(device, notification)
					}
				}
			default:
				fmt.Println("fuck you")
			}
		}
	}()

	http.HandleFunc("/clients", ListClients)

	if err := http.ListenAndServe(HTTP_BIND, nil); err != nil {
		env.Logger.Error("listen on http server error")
	}
}
