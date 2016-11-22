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

	if req.Comment == nil {
		return nil, fmt.Errorf("unfind comment")
	}

	if req.Comment.RefComment == nil {
		return nil, fmt.Errorf("unfind ref comment")
	}
	log4go.Info("on received comment reply event: user:%v", req.Comment.RefComment)

	if devices, err := device.GlobalDeviceMapper.GetDeviceByUserId(req.Comment.RefComment.UserId); err != nil {
		return nil, fmt.Errorf("get device of %v error", req.Comment.RefComment.UserId)
	} else {
		log4go.Info("pushed notifications to %d device", len(devices))
		for _, d := range devices {
			log4go.Info("push alert to [%s]", d.DeviceId)
			fcm.GlobalAppServer.PushNewCommentAlertToDevice(d, fcm.NEW_COMMENT_TYPE, fcm.MIN_NEW_COMMENT_VER)
		}
	}
	return resp, nil
}

func (s *CommentCallbackService) OnLiked(ctx context.Context, req *pb.OnLikedCallbackRequest) (*pb.EmptyMessage, error) {
	resp := &pb.EmptyMessage{}

	if req.Comment == nil{
		log4go.Info("recieve null comment")
		return resp, fmt.Errorf("null comment")
	}

	if req.Comment.Liked <= 0 {
		return resp, fmt.Errorf("0 like comment")
	}

	log4go.Info("on received comment like event: commentId:%v likenum:%d ", req.Comment.CommentId, req.Comment.Liked)

	if devices, err := device.GlobalDeviceMapper.GetDeviceByUserId(req.Comment.UserId); err != nil {
		return resp, fmt.Errorf("get device of %s error", req.Comment.UserId)
	} else {
		for _, d := range devices {
			log4go.Info("push alert to [%s]", d.DeviceId)
			fcm.GlobalAppServer.PushNewCommentAlertToDevice(d, fcm.NEW_LIKE_TYPE, fcm.MIN_LIKE_COMMENT_VER)
		}
	}

	return resp, nil
}

