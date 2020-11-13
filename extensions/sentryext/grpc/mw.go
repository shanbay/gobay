package sentrygrpcmw

import (
	"context"
	"errors"
	"log"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/shanbay/gobay/extensions/sentryext"
)

func GetOption(d *sentryext.SentryExt) grpc_recovery.Option {
	return grpc_recovery.WithRecoveryHandlerContext(
		func(ctx context.Context, err interface{}) error {
			// log err and stack trace to stdout
			log.Println(err)
			log.Println(string(debug.Stack()))

			hub := sentry.CurrentHub().Clone()
			if hub == nil {
				return errors.New("failed to get sentry hub")
			}
			if eventID := hub.RecoverWithContext(ctx, err); eventID == nil {
				return errors.New("failed sending event to Sentry")
			}
			return status.Errorf(codes.Internal, "msg: %s", err)
		},
	)
}
