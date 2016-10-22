package service

import (
	"fmt"
	pb "github.com/AgilaNews/comment/iface"

	"github.com/AgilaNews/push/device"
	"github.com/AgilaNews/push/fcm"
	"github.com/alecthomas/log4go"
	"golang.org/x/net/context"
)

type CommentCallbackService struct {
}

func NewCommentCallbackService() (*CommentCallbackService, error) {
	return &CommentCallbackService{}, nil
}

func (s *CommentCallbackService) OnReply(ctx context.Context, req *pb.OnReplyCallbackRequest) (*pb.GeneralResponse, error) {
	resp := &pb.GeneralResponse{}

	if req.Comment.RefComment == nil {
		resp.Code = pb.GeneralResponse_REQ_PARAM_ERROR
		resp.ErrorMsg = fmt.Sprintf("unfind ref comment")
		log4go.Warn("unfind ref comment")
		return resp, nil
	}
	log4go.Info("on received comment reply event: user:%v", req.Comment.RefComment)

	if devices, err := device.GlobalDeviceMapper.GetDeviceByUserId(req.Comment.RefComment.UserId); err != nil {
		resp.Code = pb.GeneralResponse_INTERNAL_ERROR
		resp.ErrorMsg = fmt.Sprintf("get device of %v error", req.Comment.RefComment.UserId)
		return resp, nil
	} else {
		log4go.Info("pushed notifications to %d device", len(devices))
		for _, d := range devices {
			log4go.Info("push alert to [%s]", d.DeviceId)
			fcm.GlobalAppServer.PushNewCommentAlertToDevice(d)
		}
	}

	resp.Code = pb.GeneralResponse_NO_ERROR
	resp.ErrorMsg = "ok"
	return resp, nil
}
