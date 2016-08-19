package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"push/env"
	"push/fcm"
	"push/task"
	"sync"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/emicklei/go-restful"
)

func main() {
	var err error
	var wg sync.WaitGroup
	var listener *net.TCPListener
	var container *restful.Container

	if err = env.Init(); err != nil {
		fmt.Println("init error : %v\n", err)
		os.Exit(-1)
	}

	if listener, container, err = NewRestfulHandler(env.Config.HttpServer.Addr); err != nil {
		fmt.Println("init resful : %v\n", err)
		os.Exit(-1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	wg.Add(3)
	go func() {
		defer wg.Done()
		fcm.GlobalAppServer.Work()
		log4go.Info("app server exists")
	}()

	go func() {
		defer wg.Done()
		if err := task.GlobalTaskManager.SyncTask(); err != nil {
			log4go.Warn("sync task info error")
		}

		task.GlobalTaskManager.Run()

		log4go.Info("task manager done")
	}()

	go func() {
		defer wg.Done()

		log4go.Info("http starts at: %v", listener.Addr())
		http.Serve(listener, container)
		log4go.Info("http server stoped")
	}()

	done := make(chan bool)
	go func() {
		wg.Wait()

		done <- true
	}()

	/*
		notification := &fcm.Notification{
			Tpl:     "2",
			NewsId:  "quz2RgKCIpY=",
			Title:   "Pinoy students win 5 medals in Romania math contest",
			Digest:  "Agila",
			Image:   "http://s1.agilanews.com/image/ZTqUvJtw6T0%3D.jpg?",
			Options: fcm.NewNotificationDefaultOptions(),
		}

		t := time.Now().Add(time.Second * 15)

		if model, err := fcm.GlobalPushManager.NewPushMessage(t, fcm.PUSH_ALL, nil, notification); err != nil {
			log4go.Warn("add notify error : %v", err)
		} else {
			fcm.GlobalPushManager.FirePushTask(model.ID)

			go func() {
				time.Sleep(time.Second * 3)
				if err := fcm.GlobalPushManager.CancelPush(model.ID); err != nil {
					log4go.Warn("cancel task error :%v", err)
				}
			}()
		}
	*/

OUTFOR:
	for {
		select {
		case <-sigs:
			task.GlobalTaskManager.Stop()
			fcm.GlobalAppServer.Stop()
			listener.Close()
			log4go.Info("get interrupt, gracefull stop")
		case <-done:
			log4go.Info("all routine done, exit")
			break OUTFOR
		}
	}

	log4go.Global.Close()
}
