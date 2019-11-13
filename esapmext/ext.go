package esapmext

import (
	"net/http"
	"net/url"

	"go.elastic.co/apm"
	"go.elastic.co/apm/transport"

	"github.com/shanbay/gobay"
)

type EsApmExt struct {
	NS     string
	app    *gobay.Application
	tracer *apm.Tracer
}

func (e *EsApmExt) Object() interface{} {
	return e.tracer
}

func (e *EsApmExt) Application() *gobay.Application {
	return e.app
}

// https://www.elastic.co/guide/en/apm/agent/go/current/configuration.html
func (e *EsApmExt) Init(app *gobay.Application) error {
	tracer := apm.DefaultTracer

	config := app.Config()
	if e.NS == "" {
		e.NS = "elastic_apm"
	}
	apmConfig := config.Sub(e.NS)
	// elastic apm load config from env by default, load env from "elastic_apm" prefix
	// make compatible with default agent behavior
	apmConfig.SetEnvPrefix(e.NS)
	apmConfig.AutomaticEnv()

	env := apmConfig.GetString("environment")
	if env == "" {
		env = config.GetString("env")
	}
	tracer.Service.Environment = env
	tracer.Service.Name = apmConfig.GetString("service_name")
	tracer.Service.Version = apmConfig.GetString("service_version")

	ts, err := transport.NewHTTPTransport()
	if err != nil {
		return err
	}
	serverUrl, err := url.Parse(apmConfig.GetString("server_url"))
	if err != nil {
		return err
	}
	ts.SetServerURL(serverUrl)
	// we can type assert this just because the apm agent uses `http.Transport` as default
	ts.Client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = !apmConfig.GetBool("verify_server_cert")
	tracer.Transport = ts

	tracer.SetCaptureBody(apm.CaptureBodyErrors)
	sampleRate := apmConfig.GetFloat64("transaction_sample_rate")
	if sampleRate <= 0 {
		sampleRate = 0.01
	}
	tracer.SetSampler(apm.NewRatioSampler(sampleRate))

	e.app = app
	e.tracer = tracer
	return nil
}

func (e *EsApmExt) Close() error {
	e.tracer.Close()
	return nil
}
