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

func (s *CommentCallbackService) OnReply(ctx context.Context, req *pb.OnReplyCallbackRequest) (*pb.EmptyMessage, error) {
	resp := &pb.EmptyMessage{}

	if req.Comment.RefComment == nil {
		return nil, fmt.Errorf("unfind ref comment")
		return resp, nil
	}
	log4go.Info("on received comment reply event: user:%v", req.Comment.RefComment)

	if devices, err := device.GlobalDeviceMapper.GetDeviceByUserId(req.Comment.RefComment.UserId); err != nil {
		return nil, fmt.Errorf("get device of %v error", req.Comment.RefComment.UserId)
	} else {
		log4go.Info("pushed notifications to %d device", len(devices))
		for _, d := range devices {
			log4go.Info("push alert to [%s]", d.DeviceId)
			fcm.GlobalAppServer.PushNewCommentAlertToDevice(d)
		}
	}

	return resp, nil
}

func (s *CommentCallbackService) OnLiked(ctx context.Context, req *pb.OnLikedCallbackRequest) (*pb.EmptyMessage, error) {
	resp := &pb.EmptyMessage{}

	return resp, nil
}
