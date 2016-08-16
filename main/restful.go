package main

import (
	"fmt"
	"net"
	"push/fcm"
	"push/task"
	"strconv"

	"github.com/alecthomas/log4go"
	"github.com/emicklei/go-restful"
)

func NewRestfulHandler(Addr string) (*net.TCPListener, error) {
	ws := new(restful.WebService)

	ws.Route(ws.GET("/push").To(getPendingPushMessage))
	ws.Route(ws.GET("/pushhistory/").To(getAllPushMessage))
	ws.Route(ws.GET("/push/{push-id}").To(getPushDetail))
	ws.Route(ws.POST("/push").To(addNewPush))

	restful.Add(ws)

	laddr, err := net.ResolveTCPAddr("tcp", Addr)
	if nil != err {
		return nil, err
	}

	listener, err := net.ListenTCP("tcp", laddr)
	if nil != err {
		return nil, err
	}

	return listener, nil

}

func getPendingPushMessage(request *restful.Request, response *restful.Response) {
	pn_str := request.QueryParameter("pn")
	page_size_str := request.QueryParameter("ps")
	pn := 10
	ps := 1
	var err error

	if len(pn_str) > 0 {
		pn, err = strconv.Atoi(pn_str)
		if err != nil {
			response.WriteErrorString(400, "pn must be integer")
			return
		}
	}
	if len(page_size_str) > 0 {
		ps, err = strconv.Atoi(page_size_str)
		if err != nil {
			response.WriteErrorString(400, "page size must be integer")
			return
		}
	}

	tasks, total_page_size := task.GlobalTaskManager.GetTasks(pn, ps-1)

	ret := struct {
		TotalPage   int              `json:"total_page"`
		CurrentPage int              `json:"currnet_page"`
		PushModels  []*fcm.PushModel `json:"pushes"`
	}{
		TotalPage:   total_page_size,
		CurrentPage: pn,
		PushModels:  make([]*fcm.PushModel, 0),
	}

	ids := make([]string, len(tasks))

	for _, task := range tasks {
		ids = append(ids, task.UserIdentifier)
	}

	if ret.PushModels, err = fcm.GlobalPushManager.BatchGetPush(ids); err != nil {
		log4go.Global.Warn("get notification error: %v", err)
		response.WriteErrorString(500, fmt.Sprint("get error : %v", err))
	} else {
		response.WriteAsJson(ret)
	}
}

func getAllPushMessage(request *restful.Request, response *restful.Response) {
	var err error
	pn_str := request.QueryParameter("pn")
	page_size_str := request.QueryParameter("ps")

	pn := 10
	ps := 0
	if len(pn_str) > 0 {
		if pn, err = strconv.Atoi(pn_str); err != nil {
			response.WriteErrorString(400, "pn error")
			return
		}
	}

	if len(page_size_str) > 0 {
		if ps, err = strconv.Atoi(page_size_str); err != nil {
			response.WriteErrorString(400, "ps error")
			return
		}
	}

	if pushes, total, err := fcm.GlobalPushManager.GetPushs(pn, ps); err != nil {
		log4go.Global.Warn("get notification error: %v", err)
		response.WriteErrorString(500, fmt.Sprint("get error : %v", err))
	} else {
		response.WriteAsJson(map[string]interface{}{
			"push":  pushes,
			"total": total,
		})
	}

}

func getPushDetail(request *restful.Request, response *restful.Response) {

}

func addNewPush(request *restful.Request, response *restful.Response) {

}
