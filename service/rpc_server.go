package service

import (
	"fmt"
	"net"

	pb "github.com/AgilaNews/comment/iface"
	"google.golang.org/grpc"
)

type CommentCallbackServer struct {
	listener  net.Listener
	rpcServer *grpc.Server
}

func NewCommentServer(addr string) (*CommentCallbackServer, error) {
	var err error

	c := &CommentCallbackServer{}

	if c.listener, err = net.Listen("tcp", addr); err != nil {
		return nil, fmt.Errorf("bind rpc %s server error", addr)
	}

	c.rpcServer = grpc.NewServer()

	pb.RegisterCallbackServiceServer(c.rpcServer, &CommentCallbackService{})

	return c, nil
}

func (c *CommentCallbackServer) Work() {
	c.rpcServer.Serve(c.listener)
}

func (c *CommentCallbackServer) Stop() {
	c.rpcServer.GracefulStop()
}
