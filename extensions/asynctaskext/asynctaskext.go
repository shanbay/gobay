package asynctaskext

import (
	"errors"
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
	if t.NS == "" {
		return errors.New("lack of NS")
	}
	t.app = app
	config := app.Config()
	config = gobay.GetConfigByPrefix(config, t.NS, true)
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

//RegisterWorkerHandlers add task handlers to worker to process task messages
func (t *AsyncTaskExt) RegisterWorkerHandlers(handlers map[string]interface{}) error {
	return t.server.RegisterTasks(handlers)
}

//StartWorker start a worker that consume task messages for queue
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

//SendTask publish task messages to broker
func (t *AsyncTaskExt) SendTask(sign *tasks.Signature) (*result.AsyncResult, error) {
	asyncResult, err := t.server.SendTask(sign)
	if err != nil {
		log.ERROR.Printf("send task failed: %v", err)
		return nil, err
	}
	return asyncResult, nil
}
