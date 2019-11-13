package asynctaskext

import (
	"os"

	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/backends/result"
	machineryConfig "github.com/RichardKnop/machinery/v1/config"
	"github.com/RichardKnop/machinery/v1/log"
	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/mitchellh/mapstructure"

	"github.com/shanbay/gobay"
)

type AsyncTaskExt struct {
	NS      string
	app     *gobay.Application
	config  *machineryConfig.Config
	server  *machinery.Server
	workers []*machinery.Worker
}

func (t *AsyncTaskExt) Object() interface{} {
	return t
}

func (t *AsyncTaskExt) Application() *gobay.Application {
	return t.app
}

func (t *AsyncTaskExt) Init(app *gobay.Application) error {
	t.app = app
	config := app.Config()
	if t.NS != "" {
		config = config.Sub(t.NS)
	}
	t.config = &machineryConfig.Config{}
	if err := config.Unmarshal(t.config, func(config *mapstructure.
	DecoderConfig) {
		config.TagName = "yaml"
	}); err != nil {
		log.FATAL.Printf("parse config error: %v", err)
	}

	server, err := machinery.NewServer(t.config)
	if err != nil {
		return err
	}
	t.server = server
	return nil
}

func (t *AsyncTaskExt) Close() error {
	for _, worker := range t.workers {
		worker.Quit()
	}
	return nil
}

//RegisterWorkerHandler add handler to worker to process message
func (t *AsyncTaskExt) RegisterWorkerHandler(name string, handler interface{}) error {
	return t.server.RegisterTask(name, handler)
}

//StartWorker start a consumer for queue
func (t *AsyncTaskExt) StartWorker(queue string) error {
	hostName, err := os.Hostname()
	if err != nil {
		log.ERROR.Printf("get host name failed: %v", err)
	}
	worker := t.server.NewWorker(hostName, 0)
	worker.Queue = queue
	t.workers = append(t.workers, worker)
	return worker.Launch()
}

//SendTask publish message to broker
func (t *AsyncTaskExt) SendTask(sign *tasks.Signature) (*result.AsyncResult, error) {
	asyncResult, err := t.server.SendTask(sign)
	if err != nil {
		log.ERROR.Printf("send task failed: %v", err)
		return nil, err
	}
	return asyncResult, nil
}
