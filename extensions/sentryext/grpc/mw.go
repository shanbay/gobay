package sentrygrpcmw

import (
	"context"

	"fmt"
	"log"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/pkg/errors"
	"github.com/shanbay/gobay/extensions/sentryext"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func GetOption(d *sentryext.SentryExt) grpc_recovery.Option {
	return grpc_recovery.WithRecoveryHandlerContext(
		func(ctx context.Context, err interface{}) error {
			switch x := err.(type) {
			case string:
				// NOTE add recover stack to err
				err = errors.New(x)
			case error:
				err = errors.New(x.Error())
			default:
				err = fmt.Errorf("%+v", err)
			}
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
