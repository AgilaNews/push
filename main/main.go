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
)

func main() {
	var err error
	var wg sync.WaitGroup
	var listener *net.TCPListener

	if err = env.Init(); err != nil {
		fmt.Println("init error : %v\n", err)
		os.Exit(-1)
	}

	if listener, err = NewRestfulHandler(env.Config.HttpServer.Addr); err != nil {
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
		task.GlobalTaskManager.Run()
		log4go.Info("task manager done")
	}()

	go func() {
		defer wg.Done()

		log4go.Info("http starts at: %v", listener.Addr())
		http.Serve(listener, nil)
		log4go.Info("http server stoped")
	}()

	done := make(chan bool)
	go func() {
		wg.Wait()

		done <- true
	}()

	notification := &fcm.Notification{
		Tpl:     "2",
		NewsId:  "quz2RgKCIpY=",
		Title:   "Pinoy students win 5 medals in Romania math contest",
		Digest:  "Agila",
		Image:   "",
		Options: fcm.NewNotificationDefaultOptions(),
	}

	t := time.Now().Add(time.Second * 1)
	if err = fcm.GlobalPushManager.AddPushTask(t, fcm.PUSH_ALL, nil, notification); err != nil {
		log4go.Warn("add notify error : %v", err)
	}

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
