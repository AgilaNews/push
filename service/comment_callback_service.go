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

	if len(req.Comment.RefComments) == 0 {
		resp.Code = pb.GeneralResponse_REQ_PARAM_ERROR
		resp.ErrorMsg = fmt.Sprintf("unfind ref comment")
		log4go.Warn("unfind ref comment")
		return resp, nil
	}
	log4go.Info("on received comment reply event: user:%v", req.Comment.RefComments[0])

	c := req.Comment.RefComments[0]
	if devices, err := device.GlobalDeviceMapper.GetDeviceByUserId(c.UserId); err != nil {
		resp.Code = pb.GeneralResponse_INTERNAL_ERROR
		resp.ErrorMsg = fmt.Sprintf("get device of %v error", c.UserId)
		return resp, nil
	} else {
		log4go.Info("pushed notifications to [device:%v]", devices)
		for _, d := range devices {
			fcm.GlobalAppServer.PushNewCommentAlertToDevice(d)
		}
	}

	resp.Code = pb.GeneralResponse_NO_ERROR
	resp.ErrorMsg = "ok"
	return resp, nil
}
