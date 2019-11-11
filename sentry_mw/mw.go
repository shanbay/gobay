package sentry_mw

import (
	"context"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/http"
	"github.com/go-openapi/runtime/middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/spf13/viper"
	"log"
)

// param config: sub viper(namespaced),
//               fileds: sentry.ClientOptions and sentryhttp.Options fields (case insensitivity)
// example yaml:
// sentry:
//   dsn: 'http://url/path'
//   environment: production
//   repanic: true  # if set true, will panic() again after recover()
func GetOpenAPIMiddleware(config *viper.Viper) (middleware.Builder, error) {
	co, o := loadOptions(config)
	if err := sentry.Init(co); err != nil {
		return nil, err
	}
	sentryHandler := sentryhttp.New(o)

	return sentryHandler.Handle, nil
}

// param config: sub viper(namespaced),
//               fileds: sentry.ClientOptions (case insensitivity)
// example yaml:
// sentry:
//   dsn: 'http://url/path'
//   environment: production
func GetGRPCHandler(config *viper.Viper) (grpc_recovery.Option, error) {
	co, _ := loadOptions(config)
	if err := sentry.Init(co); err != nil {
		return nil, err
	}
	return grpc_recovery.WithRecoveryHandlerContext(
		func(ctx context.Context, err interface{}) error {
			hub := sentry.CurrentHub().Clone()
			if eventID := hub.RecoverWithContext(ctx, err); eventID == nil {
				return errors.New("发送到sentry失败")
			}
			return nil
		},
	), nil
}

func loadOptions(config *viper.Viper) (sentry.ClientOptions, sentryhttp.Options) {
	co, o := sentry.ClientOptions{}, sentryhttp.Options{}
	config.Unmarshal(&co)
	config.Unmarshal(&o)
	if co.Dsn == "" || co.Environment == "" {
		log.Println("缺少dsn和environment配置")
	}

	return co, o
}
