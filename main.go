package main

import (
	"encoding/json"
	"github.com/mattn/go-xmpp"
	"fmt"
	"log"
	"time"
)

type DownstreamData struct {
	Body string `json:"body"`
}

type GCMDownstream struct {
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
	Messageid string `json:"message_id"`
	//CollapseKey string `json:"collapse_key,omitempty"`
	//Priority string `json:"priority,omitempty"`
	//ContentAvaliable bool `json:"content_avaliable,omitempty"`
	Notification DownstreamData `json:"notification"`
}

func main() {
	options := xmpp.Options{
		Host:     "fcm-xmpp.googleapis.com:5235",
		User:     "936821311909@gcm.googleapis.com",
		Password: "AIzaSyAj_3nEtK9bhdIoNTCXliFrc26zGO5GsPg",
		NoTLS:    false,
		Debug:    true,
	}

	client, err := options.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			chat, err := client.Recv()
			if err != nil {
				log.Fatal(err)
			}

			switch v := chat.(type) {
			case xmpp.Chat:
				fmt.Println("Chatting")
				fmt.Println(v.Remote, v.Text)
				fmt.Println("#########################")
			case xmpp.Presence:
				fmt.Println("Presence")
				fmt.Println(v.From, v.Show)
				fmt.Println("#########################")
			}
		}
	}()

	downstream := GCMDownstream{
		To: "/topics/news",
		//		Condition: "news in topics",
		Messageid: "123",
		Notification: DownstreamData{
			Body: "Hello World",
		},
	}

	body, err := json.Marshal(downstream)
	if err != nil {
		log.Fatal(err)
	}

	msg := "<message><gcm xmlns=\"google:mobile:data\">" + string(body) + "</gcm></message>"
	fmt.Println("send " + msg)

	client.SendOrg(msg)

	for {
		time.Sleep(1)
	}
}
