package main

import (
	"encoding/json"
	"fmt"
	"github.com/mattn/go-xmpp"
	"log"
	"time"
)

type GCMDownstreamData struct {
	Type   string `json:"type"`
	PushId string `json:"push_id"`
	Tpl    string `json:"tpl"`
	Title  string `json:"title"`
	Digest string `json:"digest"`
	Img    string `json:"img"`
	NewsId string `json:"news_id"`
}

type GCMDownstream struct {
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
	Messageid string `json:"message_id"`
	//CollapseKey string `json:"collapse_key,omitempty"`
	//Priority string `json:"priority,omitempty"`
	//ContentAvaliable bool `json:"content_avaliable,omitempty"`
	//	Notification DownstreamData `json:"notification"`
	Data GCMDownstreamData `json:"data"`
}

func main() {
	options := xmpp.Options{
		Host:     "fcm-xmpp.googleapis.com:5235",
		User:     "1066815885426@gcm.googleapis.com",
		Password: "AIzaSyBMK2JittPIQI489utC3QVIOE-VSa4djwk",
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
		To:        "/topics/notification",
		Messageid: "123",
		Data: GCMDownstreamData{
			Type:   "1",
			PushId: "123",
			Tpl:    "2",
			Title:  "animal zhan",
			Digest: "lianzhan is animal",
			Img:    "http://img.agilanews.info/image/SohE2st5Dag=.jpg?p=t=180x180|q=45",
			NewsId: "VRGFHuVr9y0=",
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
