package sentryext

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/shanbay/gobay"
)

// SentryExt sentry OpenAPI extension
type SentryExt struct {
	NS  string
	app *gobay.Application
}

// Init implements Extension interface
func (d *SentryExt) Init(app *gobay.Application) error {
	d.app = app
	config := app.Config()
	if d.NS != "" {
		config = config.Sub(d.NS)
		config.SetEnvPrefix(d.NS)
	}
	config.AutomaticEnv()
	co := sentry.ClientOptions{}
	if err := config.Unmarshal(&co); err != nil {
		return err
	}
	if co.Dsn == "" || co.Environment == "" {
		return errors.New("lack dsn or environment")
	}
	if err := sentry.Init(co); err != nil {
		return err
	}
	return nil
}

// Close implements Extension interface
func (d *SentryExt) Close() error {
	return nil
}

// Object implements Extension interface
func (d *SentryExt) Object() interface{} {
	return d
}

// Application implements Extension interface
func (d *SentryExt) Application() *gobay.Application {
	return d.app
}
