/*
How can I check that a running asynctask worker can otherwise process a new message within 5 seconds (health check)?
- `curl 127.0.0.1:5000/health?timeout=5&queue=gobay.task_sub`
- `curl 127.0.0.1:5000/health?timeout=5` **default queue**
*/
package asynctaskext

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/backends/result"
	machineryConfig "github.com/RichardKnop/machinery/v1/config"
	"github.com/RichardKnop/machinery/v1/log"
	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/shanbay/gobay"
)

const (
	healthCheckTaskName = "gobay-asynctask-health-check"
)

type AsyncTaskExt struct {
	NS      string
	app     *gobay.Application
	config  *machineryConfig.Config
	server  *machinery.Server
	workers []*machinery.Worker

	lock                    sync.Mutex
	healthCheckCompleteChan chan string
	healthHandlerRegistered bool
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
	t.healthCheckCompleteChan = make(chan string, 1)
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
	return t.registerHealthCheck()
}

func (t *AsyncTaskExt) Close() error {
	for _, worker := range t.workers {
		worker.Quit()
	}
	return nil
}

//RegisterWorkerHandler add task handler to worker to process task messages
func (t *AsyncTaskExt) RegisterWorkerHandler(name string, handler interface{}) error {
	return t.server.RegisterTask(name, handler)
}

//RegisterWorkerHandlers add task handlers to worker to process task messages
func (t *AsyncTaskExt) RegisterWorkerHandlers(handlers map[string]interface{}) error {
	return t.server.RegisterTasks(handlers)
}

//StartWorker start a worker that consume task messages for queue
func (t *AsyncTaskExt) StartWorker(queue string, concurrency int, enableHealthCheck bool) error {
	t.lock.Lock()

	if queue == "" {
		queue = t.config.DefaultQueue
	}
	tag := t.genConsumerTag(queue)
	worker := t.server.NewWorker(tag, concurrency)
	worker.Queue = queue
	t.workers = append(t.workers, worker)

	// run health check http server
	if enableHealthCheck && !t.healthHandlerRegistered {
		t.healthHandlerRegistered = true
		healthSrv := http.Server{Addr: ":5000"}
		http.Handle("/health", http.HandlerFunc(t.healthHttpHandler))
		go func() {
			if err := healthSrv.ListenAndServe(); err != nil {
				log.FATAL.Printf("error when start prometheus server: %v\n", err)
			}
		}()
	}

	t.lock.Unlock()
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

//SendTask publish task messages with context to broker
func (t *AsyncTaskExt) SendTaskWithContext(ctx context.Context, sign *tasks.Signature) (*result.AsyncResult, error) {
	asyncResult, err := t.server.SendTaskWithContext(ctx, sign)
	if err != nil {
		log.ERROR.Printf("send task with context failed: %v", err)
		return nil, err
	}
	return asyncResult, nil
}

func (t *AsyncTaskExt) genConsumerTag(queue string) string {
	hostName, err := os.Hostname()
	if err != nil {
		log.ERROR.Printf("get host name failed: %v", err)
	}
	return fmt.Sprintf("%s@%s", queue, hostName)
}

func (t *AsyncTaskExt) registerHealthCheck() error {
	return t.server.RegisterTask(healthCheckTaskName, func(healthCheckUUID string) error {
		select {
		case t.healthCheckCompleteChan <- healthCheckUUID: // success and send uuid
			return nil
		case <-time.After(5 * time.Second):
			return fmt.Errorf("send health check result error: %v", healthCheckUUID)
		}
	})
}

// Send a health check. Expect it to be processed within taskExecutionTimeout, otherwise it is considered unhealthy and return err
func (t *AsyncTaskExt) checkHealth(consumerTag string, taskExecutionTimeout time.Duration) error {
	// clear channel
	select {
	case <-t.healthCheckCompleteChan:
	default:
	}

	broker := t.server.GetBroker()
	healthCheckUUID, err := uuid.NewUUID()
	if err != nil {
		return err
	}
	if err := broker.PublishToLocal(consumerTag, &tasks.Signature{
		UUID: healthCheckUUID.String(),
		Name: healthCheckTaskName,
		Args: []tasks.Arg{
			{Type: "string", Value: healthCheckUUID.String()},
		},
	}, 5*time.Second); err != nil {
		return err
	}

	// wait for task execution success
	select {
	case successUUID := <-t.healthCheckCompleteChan:
		if successUUID == healthCheckUUID.String() {
			return nil
		}
	case <-time.After(taskExecutionTimeout):
	}
	return fmt.Errorf("health check execution fail: %v", healthCheckUUID.String())
}

// HTTP handler that triggers the health check
func (t *AsyncTaskExt) healthHttpHandler(w http.ResponseWriter, r *http.Request) {
	// get params
	params := r.URL.Query()
	if len(params["timeout"]) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("no timeout")); err != nil {
			panic(err)
		}
		return
	}
	timeoutInSeconds, err := strconv.Atoi(params["timeout"][0])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err = w.Write([]byte(err.Error())); err != nil {
			panic(err)
		}
		return
	}
	queue := t.config.DefaultQueue
	if len(params["queue"]) == 1 && params["queue"][0] != "" {
		queue = params["queue"][0]
	}
	consumerTag := t.genConsumerTag(queue)

	// send health check
	if err := t.checkHealth(consumerTag, time.Duration(timeoutInSeconds)*time.Second); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err = w.Write([]byte(err.Error())); err != nil {
			panic(err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte("OK")); err != nil {
		panic(err)
	}
}
