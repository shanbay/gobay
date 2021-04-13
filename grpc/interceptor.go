package gobay_grpc

import (
	"context"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/shanbay/gobay"
	"google.golang.org/grpc"
)

type GobayAppKey string

const (
	appkey GobayAppKey = "gobay_app"
)

func UnaryCurrentAPPServerInterceptor(app *gobay.Application) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx = context.WithValue(ctx, appkey, app)
		return handler(ctx, req)
	}
}

func StreamCurrentAPPServerInterceptor(app *gobay.Application) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		ctx := stream.Context()
		ctx = context.WithValue(ctx, appkey, app)
		wrappedStream := grpc_middleware.WrapServerStream(stream)
		wrappedStream.WrappedContext = ctx
		err = handler(srv, wrappedStream)
		return err
	}
}

func GetCurrentAPP(ctx context.Context) *gobay.Application {
	if app, ok := ctx.Value(appkey).(*gobay.Application); ok {
		return app
	}
	return nil
}
