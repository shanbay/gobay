package ent_mw

import (
	"context"
	"github.com/shanbay/gobay/extensions/entext"
	gobay_grpc "github.com/shanbay/gobay/grpc"
	"google.golang.org/grpc"
)

func GetEntUnaryMw(e *entext.EntExt) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, rpcerr := handler(ctx, req)
		err := rpcerr
		if e.IsNotFound != nil && err != nil && e.IsNotFound(rpcerr) {
			err = gobay_grpc.NotFoundError
		}
		if e.IsConstraintFailure != nil && err != nil && e.IsConstraintFailure(rpcerr) {
			err = gobay_grpc.AlreadyExistsError
		}
		return resp, err
	}
}

func GetEntStreamMw(e *entext.EntExt) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		rpcerr := handler(srv, ss)
		err := rpcerr
		if e.IsNotFound != nil && err != nil && e.IsNotFound(rpcerr) {
			err = gobay_grpc.NotFoundError
		}
		if e.IsConstraintFailure != nil && err != nil && e.IsConstraintFailure(rpcerr) {
			err = gobay_grpc.AlreadyExistsError
		}
		return err
	}
}
