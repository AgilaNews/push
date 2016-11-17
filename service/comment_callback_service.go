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
			fcm.GlobalAppServer.PushNewCommentAlertToDevice(d, fcm.NEW_COMMENT_TYPE)
		}
	}

	resp.Code = pb.GeneralResponse_NO_ERROR
	resp.ErrorMsg = "ok"
	return resp, nil
}

func (s *CommentCallbackService) OnLike(ctx context.Context, req *pb.OnLikedCallbackRequest) (*pb.GeneralResponse, error) {
	resp := &pb.GeneralResponse{}

	if req.Comment.Liked <= 0 {
		resp.Code = pb.GeneralResponse_REQ_PARAM_ERROR
		resp.ErrorMsg = fmt.Sprintf("0 like comment")
		log4go.Warn("0 like comment")
		return resp, nil
	}
	log4go.Info("on received comment like event: commentId:%v likenum:%d ", req.Comment.ID, req.Comment.Liked)

	if devices, err := device.GlobalDeviceMapper.GetDeviceByUserId(req.Comment.UserId); err != nil {
		resp.Code = pb.GeneralResponse_INTERNAL_ERROR
		resp.ErrorMsg = fmt.Sprintf("get device of %v error", req.Comment.UserId)
		return resp, nil
	} else {
		log4go.Info("pushed notifications to %d device", len(devices))
		for _, d := range devices {
			log4go.Info("push alert to [%s]", d.DeviceId)
			fcm.GlobalAppServer.PushNewCommentAlertToDevice(d, fcm.NEW_LIKE_TYPE)
		}
	}

	return resp, nil
}

func (s *CommentCallbackService) OnLiked(ctx context.Context, req *pb.OnLikedCallbackRequest) (*pb.EmptyMessage, error) {
	resp := &pb.EmptyMessage{}

	return resp, nil
}
