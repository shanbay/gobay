package sentrygrpcmw

import (
	"context"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/shanbay/gobay/sentryext"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func GetOption(d *sentryext.SentryExt) grpc_recovery.Option {
	return grpc_recovery.WithRecoveryHandlerContext(
		func(ctx context.Context, err interface{}) error {
			hub := sentry.CurrentHub().Clone()
			if eventID := hub.RecoverWithContext(ctx, err); eventID == nil {
				return errors.New("failed sending event to Sentry")
			}
			return status.Errorf(codes.Internal, "msg: %s", err)
		},
	)
}
