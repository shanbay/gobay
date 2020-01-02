package esapmext

import (
	"net/http"
	"net/url"

	"go.elastic.co/apm"
	"go.elastic.co/apm/transport"

	"github.com/shanbay/gobay"
)

type EsApmExt struct {
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

	config := app.GetConfigByPrefix("elastic_", false)
	if !config.GetBool("elastic_apm_enable") {
		return nil
	}
	// elastic apm load config from env by default
	// use `elastic_apm` as config prefix so we can make compatible with default agent behavior
	env := config.GetString("elastic_apm_environment")
	if env == "" {
		env = app.Env()
	}

	tracer := apm.DefaultTracer
	tracer.Service.Environment = env
	tracer.Service.Name = config.GetString("elastic_apm_service_name")
	tracer.Service.Version = config.GetString("elastic_apm_service_version")

	ts, err := transport.NewHTTPTransport()
	if err != nil {
		return err
	}
	serverUrl, err := url.Parse(config.GetString("elastic_apm_server_url"))
	if err != nil {
		return err
	}
	ts.SetServerURL(serverUrl)
	// we can type assert this just because the apm agent uses `http.Transport` as default
	ts.Client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = !config.GetBool("elastic_apm_verify_server_cert")
	tracer.Transport = ts

	tracer.SetCaptureBody(apm.CaptureBodyErrors)
	sampleRate := config.GetFloat64("elastic_apm_transaction_sample_rate")
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
