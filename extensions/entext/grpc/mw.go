package ent_mw

import (
	"context"
	"github.com/shanbay/gobay/extensions/entext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	NotFoundError      = status.Error(codes.NotFound, "Not Found")
	AlreadyExistsError = status.Error(codes.AlreadyExists, "Already Exists")
)

func GetEntUnaryMw(e *entext.EntExt) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, rpcerr := handler(ctx, req)
		err := rpcerr
		if e.IsNotFound != nil && e.IsNotFound(rpcerr) {
			err = NotFoundError
		}
		if e.IsConstraintError != nil && e.IsConstraintError(rpcerr) {
			err = AlreadyExistsError
		}
		return resp, err
	}
}

func GetEntStreamMw(e *entext.EntExt) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		rpcerr := handler(srv, ss)
		err := rpcerr
		if e.IsNotFound != nil && e.IsNotFound(rpcerr) {
			err = NotFoundError
		}
		if e.IsConstraintError != nil && e.IsConstraintError(rpcerr) {
			err = AlreadyExistsError
		}
		return err
	}
}
