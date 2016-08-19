package main

import (
	"encoding/json"
	"fmt"
	"net"
	"push/device"
	"push/env"
	"push/fcm"
	"push/task"
	"strconv"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
	"github.com/jinzhu/gorm"
)

const (
	ERR_PARAM      = 40001
	ERR_NON_EXISTS = 40401
	ERR_INTERNAL   = 50001
)

type JsonResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
}

type PushForm struct {
	Tpl         string                   `json:"template" description:"template, currently ,we just have '2'"`
	NewsId      string                   `json:"news_id" description:"news id"`
	Title       string                   `json:"title" description:"title, main message is notification"`
	Digest      string                   `json:"digest" description:"digest, second line of notification"`
	Image       string                   `json:"image" description:"image of notification"`
	DeliverType fcm.PushType             `json:"deliver_type,number" description:"delivery type"`
	Options     *fcm.NotificationOptions `json:"options"`

	Instant   bool               `json:"instant"`
	PlanTime  int64              `json:"plan_time"`
	Condition *fcm.PushCondition `json:"condition"`
}

type PushListResponse struct {
	Total  int              `json:"total" description:"total push messages"`
	Pushes []*fcm.PushModel `json:"pushes" description:"push messages list"`
}

func WriteJsonSuccess(r *restful.Response, c interface{}) {

	if c == nil {
		c = JsonResponse{Message: "ok"}
	}

	r.WriteHeaderAndJson(200, c, restful.MIME_JSON)
}

func WriteJsonError(r *restful.Response, status, code int, message string) {
	e := JsonResponse{
		Code:    code,
		Message: message,
	}

	r.WriteHeaderAndJson(status, e, restful.MIME_JSON)
}

func NewRestfulHandler(Addr string) (*net.TCPListener, *restful.Container, error) {
	container := restful.NewContainer()

	cors := restful.CrossOriginResourceSharing{
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedDomains: []string{"localhost"},
	}

	container.Filter(cors.Filter)

	ws := new(restful.WebService)
	ws.Path("/push").
		Doc("push management").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	ws.Route(ws.GET("/").To(getAllPushMessage).
		Doc("get histroy of push").
		Param(ws.QueryParameter("start", "pagination start").DataType("int").Required(false).DefaultValue("1")).
		Param(ws.QueryParameter("length", "pagnination length").DataType("int").Required(false).DefaultValue("10")).
		Param(ws.QueryParameter("filter", "filter k-v (json format)").Description("support key is news_id, id").DataType("string").Required(false)).
		Writes(PushListResponse{}))

	ws.Route(ws.POST("/").To(newPush).
		Doc("create push task (not instant push, just create new one) of broadcast(1) | multicast(2) type").
		Reads(PushForm{}).
		Writes(fcm.PushModel{}).
		Returns(500, "internal error", nil).
		Returns(400, "parameter error", nil),
	)

	ws.Route(ws.DELETE("/detail/{push-id}").To(cancelPush).
		Doc("cancel task").
		Writes(JsonResponse{}).
		Returns(500, "internal error", nil).
		Returns(400, "parameter error", nil),
	)

	ws.Route(ws.POST("/detail/{push-id}").To(updatePush).
		Doc("alter push task").
		Reads(PushForm{}).
		Writes(fcm.PushModel{}).
		Returns(500, "internal error", nil).
		Returns(400, "parameter error", nil),
	)

	ws.Route(ws.PUT("/detail/{push-id}").To(firePush).
		Doc("fired push message").
		Param(ws.PathParameter("push-id", "push id got from the list").DataType("int").Required(true)).
		Writes(JsonResponse{}).
		Returns(500, "internal error", nil).
		Returns(400, "parameter error", nil).
		Returns(409, "invalid task status", nil),
	)

	container.Add(ws)

	ws = new(restful.WebService)
	ws.Path("/device/").
		Doc("device management").
		Consumes("restful.MIME_JSON").
		Produces("restful.MIME_JSON")

	ws.Route(ws.GET("/{device-id}").To(getDevice).
		Doc("get device by did").
		Writes(device.Device{}))

	config := swagger.Config{
		WebServices:    container.RegisteredWebServices(),
		WebServicesUrl: "http://" + env.Config.HttpServer.Addr,
		ApiPath:        "/apidocs.json/",

		SwaggerPath:     "/apidocs/",
		SwaggerFilePath: env.Config.HttpServer.SwaggerPath}

	swagger.RegisterSwaggerService(config, container)

	laddr, err := net.ResolveTCPAddr("tcp", Addr)
	if nil != err {
		return nil, nil, err
	}

	listener, err := net.ListenTCP("tcp", laddr)
	if nil != err {
		return nil, nil, err
	}

	return listener, container, nil

}

func getPendingPushMessage(request *restful.Request, response *restful.Response) {
	pn_str := request.QueryParameter("start")
	page_size_str := request.QueryParameter("length")
	pn := 10
	ps := 1
	var err error

	if len(pn_str) > 0 {
		pn, err = strconv.Atoi(pn_str)
		if err != nil {
			WriteJsonError(response, 400, ERR_PARAM, "pn must be integer")
			return
		}
	}
	if len(page_size_str) > 0 {
		ps, err = strconv.Atoi(page_size_str)
		if err != nil {
			WriteJsonError(response, 400, ERR_PARAM, "page size must be integer")
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
		WriteJsonError(response, 500, ERR_INTERNAL, fmt.Sprint("get error : %v", err))
	} else {
		WriteJsonSuccess(response, ret)
	}
}

func getAllPushMessage(request *restful.Request, response *restful.Response) {
	var err error
	var filters map[string]string

	start_str := request.QueryParameter("start")
	length_str := request.QueryParameter("length")
	filter_str := request.QueryParameter("filter")

	if len(filter_str) > 0 {
		if err := json.Unmarshal([]byte(filter_str), &filters); err != nil {
			WriteJsonError(response, 400, ERR_PARAM, "filters must be json k-v format")
			return
		}
	}

	start := 0
	length := 2

	if len(start_str) > 0 {
		if start, err = strconv.Atoi(start_str); err != nil {
			WriteJsonError(response, 400, ERR_PARAM, "start error")
			return
		}
		if start < 0 {
			start = 0
		}
	}

	if len(length_str) > 0 {
		if length, err = strconv.Atoi(length_str); err != nil {
			WriteJsonError(response, 400, ERR_PARAM, "ps error")
			return
		}
		if length < 0 {
			length = 2
		}
	}

	if pushes, total, err := fcm.GlobalPushManager.GetPushs(start, length, filters); err != nil {
		log4go.Global.Warn("get notification error: %v", err)
		WriteJsonError(response, 500, ERR_INTERNAL, fmt.Sprint("get error : %v", err))
	} else {
		WriteJsonSuccess(response, PushListResponse{
			Pushes: pushes,
			Total:  total,
		})
	}
}

func updatePush(request *restful.Request, response *restful.Response) {
	var push_id int
	var err error
	if push_id, err = strconv.Atoi(request.PathParameter("push-id")); err != nil {
		WriteJsonError(response, 400, ERR_PARAM, "push-id must be int")
		return
	}
	form := &PushForm{}

	json_accessor := restful.NewEntityAccessorJSON(restful.MIME_JSON)
	if err := json_accessor.Read(request, form); err != nil {
		WriteJsonError(response, 400, ERR_PARAM, fmt.Sprintf("pushModel format error: %v", err))
		return
	}

	notification := &fcm.Notification{
		Tpl:     form.Tpl,
		NewsId:  form.NewsId,
		Title:   form.Title,
		Digest:  form.Digest,
		Image:   form.Image,
		Options: form.Options,
	}

	if model, err := fcm.GlobalPushManager.UpdatePush(push_id, time.Unix(form.PlanTime, 0), form.Condition, notification); err != nil {
		WriteJsonError(response, 500, ERR_INTERNAL, fmt.Sprintf("update error : %v", err))
		log4go.Global.Warn("update push task error: %v", err)
	} else {
		WriteJsonSuccess(response, model)
	}
}

func firePush(request *restful.Request, response *restful.Response) {
	var push_id int
	var err error
	if push_id, err = strconv.Atoi(request.PathParameter("push-id")); err != nil {
		WriteJsonError(response, 400, ERR_PARAM, "push-id must be int")
		return
	}

	if err := fcm.GlobalPushManager.FirePushTask(uint(push_id)); err != nil {
		if err == gorm.ErrRecordNotFound {
			WriteJsonError(response, 404, ERR_NON_EXISTS, "push not found")
		} else {
			WriteJsonError(response, 500, ERR_INTERNAL, fmt.Sprintf("fire push error: %v", err))
		}
	} else {
		WriteJsonSuccess(response, nil)
	}
}

func cancelPush(request *restful.Request, response *restful.Response) {
	var push_id int
	var err error
	if push_id, err = strconv.Atoi(request.PathParameter("push-id")); err != nil {
		WriteJsonError(response, 400, ERR_PARAM, "push-id must be int")
		return
	}

	if err := fcm.GlobalPushManager.CancelPush(uint(push_id)); err != nil {
		if err == gorm.ErrRecordNotFound {
			WriteJsonError(response, 404, ERR_NON_EXISTS, "push not found")
		} else {
			WriteJsonError(response, 500, ERR_INTERNAL, fmt.Sprintf("fire push error: %v", err))
		}
	} else {
		WriteJsonSuccess(response, nil)
	}
}

func newPush(request *restful.Request, response *restful.Response) {
	form := &PushForm{}

	json_accessor := restful.NewEntityAccessorJSON(restful.MIME_JSON)
	if err := json_accessor.Read(request, form); err != nil {
		WriteJsonError(response, 400, ERR_PARAM, fmt.Sprintf("pushModel format error: %v", err))
		return
	}

	notification := &fcm.Notification{
		Tpl:     form.Tpl,
		NewsId:  form.NewsId,
		Title:   form.Title,
		Digest:  form.Digest,
		Image:   form.Image,
		Options: form.Options,
	}

	if form.Instant {
		if form.DeliverType != fcm.PUSH_MULTICAST {
			WriteJsonError(response, 400, ERR_PARAM, "instant push must be multicast type")
			return
		}

		if len(form.Condition.Devices) == 0 {
			WriteJsonError(response, 400, ERR_PARAM, "multicast type must have at least one deivce")
			return
		}

		if err := fcm.GlobalPushManager.InstantMulticast(form.Condition.Devices, notification); err != nil {
			WriteJsonError(response, 500, ERR_PARAM, fmt.Sprintf("push to devices error : %v", err))
		} else {
			log4go.Info("push to device success")
			WriteJsonSuccess(response, nil)
		}
	} else {
		if model, err := fcm.GlobalPushManager.NewPushMessage(time.Unix(form.PlanTime, 0),
			form.DeliverType, form.Condition, notification); err != nil {
			WriteJsonError(response, 500, ERR_INTERNAL, fmt.Sprintf("update error : %v", err))
			log4go.Global.Warn("add push task error: %v", err)
		} else {
			WriteJsonSuccess(response, model)
		}
	}
}

func getDevice(request *restful.Request, response *restful.Response) {
	device_id := request.PathParameter("device-id")

	if d, err := device.GlobalDeviceMapper.GetDeviceById(device_id); err != nil {
		if err == device.ErrDeviceNotFound {
			WriteJsonError(response, 404, ERR_PARAM, "device not found")
		} else {
			WriteJsonError(response, 500, ERR_INTERNAL, fmt.Sprintf("get device error : %v", err))
		}
	} else {
		WriteJsonSuccess(response, d)
	}
}