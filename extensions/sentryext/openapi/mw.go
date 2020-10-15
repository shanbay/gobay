package sentryopenapimw

import (
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-openapi/runtime/middleware"

	"github.com/shanbay/gobay/extensions/sentryext"
)

func GetMiddleWare(d *sentryext.SentryExt) (middleware.Builder, error) {
	config := d.Config()
	o := sentryhttp.Options{}
	if err := config.Unmarshal(&o); err != nil {
		return nil, err
	}
	if !config.IsSet("repanic") {
		o.Repanic = true
	}
	handler := sentryhttp.New(o)
	return handler.Handle, nil
}
