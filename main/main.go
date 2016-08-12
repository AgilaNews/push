package main

import (
	"fmt"
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

	if err = env.Init(); err != nil {
		fmt.Println("init error : %v\n", err)
		os.Exit(-1)
	}

	var wg sync.WaitGroup

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	wg.Add(2)
	go func() {
		fcm.GlobalAppServer.Work()
		log4go.Info("app server exists")
		wg.Done()
	}()

	go func() {
		task.GlobalTaskManager.Run()
		log4go.Info("task manager done")
		wg.Done()
	}()

	done := make(chan bool)
	go func() {
		wg.Wait()

		done <- true
	}()

	//TODO test
	notification := &fcm.Notification{
		Tpl:     "2",
		NewsId:  "quz2RgKCIpY=",
		Title:   "Pinoy students win 5 medals in Romania math contest",
		Digest:  "Agila",
		Image:   "",
		Options: fcm.NewNotificationDefaultOptions(),
	}

	t := time.Now().Add(time.Second * 5)
	if err = fcm.GlobalPushManager.AddNotificationTask(t, fcm.PUSH_ALL, notification); err != nil {
		log4go.Warn("add notify error : %v", err)
	}

OUTFOR:
	for {
		select {
		case <-sigs:
			task.GlobalTaskManager.Stop()
			fcm.GlobalAppServer.Stop()
			log4go.Info("get interrupt, gracefull stop")
		case <-done:
			log4go.Info("all routine done, exit")
			break OUTFOR
		}
	}

	log4go.Global.Close()
}
