package sentryopenapimw

import (
	"github.com/getsentry/sentry-go/http"
	"github.com/go-openapi/runtime/middleware"
	"github.com/shanbay/gobay/sentryext"
)

func GetMiddleWare(d *sentryext.SentryExt) (middleware.Builder, error) {
	config := d.Config()
	o := sentryhttp.Options{}
	if err := config.Unmarshal(&o); err != nil {
		return nil, err
	}
	handler := sentryhttp.New(o)
	return handler.Handle, nil
}
