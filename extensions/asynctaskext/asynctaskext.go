/*
健康检查（watchdog 式存活探针）：

- `curl 127.0.0.1:5000/health?queue=gobay.task_sub`
- `curl 127.0.0.1:5000/health` **default queue**

判定的是「消费循环还在不在转」，不是「任务跑得快不快」：心跳新鲜即健康；
心跳停止但满载说明拉取循环因 deliveries 填满而正常阻塞，同样不判死；只有
心跳停止且仍有空闲槽位，才说明消费循环卡死、只能靠重启恢复。
timeout / queue 两个 URL 参数仍被接收，timeout 已不参与判定。
*/
package asynctaskext

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/backends/result"
	machineryConfig "github.com/RichardKnop/machinery/v1/config"
	"github.com/RichardKnop/machinery/v1/log"
	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/mitchellh/mapstructure"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/observability"
)

type AsyncTaskExt struct {
	NS      string
	app     *gobay.Application
	config  *machineryConfig.Config
	server  *machinery.Server
	workers []*machinery.Worker

	lock                    sync.Mutex
	healthHandlerRegistered bool

	monitorEnabled  bool
	taskStartTimes  map[string]time.Time
	taskStartTimesM sync.Mutex

	healthMu     sync.RWMutex
	workerHealth map[string]*workerHealthStat
}

// healthStaleThreshold 是心跳新鲜度阈值 X。量纲取自 machinery redis broker 的
// BLPOP 轮询周期（NormalTasksPollPeriod，默认 1s）——拉取循环空闲时约每秒 tick
// 一次，60s 即 60 倍余量。它与业务任务时长无关，因此无需各服务校准。
// 声明为 var 而非 const，以便测试临时覆盖后还原。
var healthStaleThreshold = 60 * time.Second

// workerHealthStat 保存单个 worker 的 watchdog 判定状态。
type workerHealthStat struct {
	lastTick    int64 // atomic, UnixNano: 拉取循环最近一次迭代的时间
	inFlight    int64 // atomic: 正在执行的任务数
	concurrency int64 // 并发上限（创建后只读），已按 machinery 的 concurrency < 1 规则归一
}

// healthy 实现 watchdog 判定：心跳新鲜即健康；心跳停止时，满载说明拉取循环是
// 因 deliveries 缓冲填满而正常阻塞（在忙），此时主动选择不杀。只有「心跳停止
// 且仍有空闲槽位」才能确证消费循环卡死。
func (s *workerHealthStat) healthy(now time.Time) bool {
	if now.Sub(time.Unix(0, atomic.LoadInt64(&s.lastTick))) < healthStaleThreshold {
		return true
	}
	return atomic.LoadInt64(&s.inFlight) >= s.concurrency
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

	if config.GetBool("monitor_enable") {
		t.monitorEnabled = true
		t.taskStartTimes = make(map[string]time.Time)
	}

	return nil
}

func (t *AsyncTaskExt) Close() error {
	for _, worker := range t.workers {
		worker.Quit()
	}
	return nil
}

// RegisterWorkerHandler add task handler to worker to process task messages
func (t *AsyncTaskExt) RegisterWorkerHandler(name string, handler interface{}) error {
	return t.server.RegisterTask(name, handler)
}

// RegisterWorkerHandlers add task handlers to worker to process task messages
func (t *AsyncTaskExt) RegisterWorkerHandlers(handlers map[string]interface{}) error {
	return t.server.RegisterTasks(handlers)
}

// StartWorker start a worker that consume task messages for queue
func (t *AsyncTaskExt) StartWorker(queue string, concurrency int, enableHealthCheck bool) error {
	t.lock.Lock()

	if queue == "" {
		queue = t.config.DefaultQueue
	}
	tag := t.genConsumerTag(queue)
	worker := t.server.NewWorker(tag, concurrency)
	worker.Queue = queue
	t.workers = append(t.workers, worker)

	// 并发数按 machinery broker 内部规则归一：它在 concurrency < 1 时会用
	// runtime.NumCPU()*2，此处不复制同一兜底的话，满载判定将永不成立。
	effectiveConcurrency := concurrency
	if effectiveConcurrency < 1 {
		effectiveConcurrency = runtime.NumCPU() * 2
	}
	stat := &workerHealthStat{concurrency: int64(effectiveConcurrency)}
	// 启动期初始化：零值等同「心跳停止」，叠加 inFlight == 0 会让探针在拉取
	// 循环转起来之前就把容器判为不健康。
	atomic.StoreInt64(&stat.lastTick, time.Now().UnixNano())
	t.healthMu.Lock()
	if t.workerHealth == nil {
		t.workerHealth = make(map[string]*workerHealthStat)
	}
	t.workerHealth[tag] = stat
	t.healthMu.Unlock()

	// 心跳挂载点：machinery 的拉取循环每轮迭代都会调用 PreConsumeHandler。
	// 必须返回 true，否则会跳过取任务。
	worker.SetPreConsumeHandler(func(*machinery.Worker) bool {
		atomic.StoreInt64(&stat.lastTick, time.Now().UnixNano())
		return true
	})
	worker.SetPreTaskHandler(func(sig *tasks.Signature) {
		atomic.AddInt64(&stat.inFlight, 1)
		if t.monitorEnabled {
			t.recordTaskStart(sig)
		}
	})
	worker.SetPostTaskHandler(func(sig *tasks.Signature) {
		if t.monitorEnabled {
			t.recordTaskDuration(sig, queue)
		}
		atomic.AddInt64(&stat.inFlight, -1)
	})

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

// SendTask publish task messages to broker
func (t *AsyncTaskExt) SendTask(sign *tasks.Signature) (*result.AsyncResult, error) {
	asyncResult, err := t.server.SendTask(sign)
	if err != nil {
		log.ERROR.Printf("send task failed: %v", err)
		return nil, err
	}
	return asyncResult, nil
}

// SendTask publish task messages with context to broker
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

// recordTaskStart 记录任务开始处理的时间点，按 signature.UUID 索引。
func (t *AsyncTaskExt) recordTaskStart(sig *tasks.Signature) {
	t.taskStartTimesM.Lock()
	defer t.taskStartTimesM.Unlock()
	t.taskStartTimes[sig.UUID] = time.Now()
}

// recordTaskDuration 从 taskStartTimes 弹出起点算 duration，Observe 到
// observability.AsyncTaskDurationSeconds。status 固定写死 "unknown"——machinery
// 的 SetPostTaskHandler 全局 hook 拿不到该次调用的 error，无法在并发 worker 下
// 精确归因到具体任务，为避免引入 reflect.MakeFunc 包装 handler 的复杂度和风险，
// 明确放弃精确 status（详见 plan §1.3）。
func (t *AsyncTaskExt) recordTaskDuration(sig *tasks.Signature, queue string) {
	t.taskStartTimesM.Lock()
	start, ok := t.taskStartTimes[sig.UUID]
	if ok {
		delete(t.taskStartTimes, sig.UUID)
	}
	t.taskStartTimesM.Unlock()
	if !ok {
		return
	}
	observability.AsyncTaskDurationSeconds.WithLabelValues(sig.Name, queue, "unknown").
		Observe(time.Since(start).Seconds())
}

// HTTP handler that triggers the health check
func (t *AsyncTaskExt) healthHttpHandler(w http.ResponseWriter, r *http.Request) {
	// timeout 参数保留接收但忽略：判定不再涉及任何排队等待，缺失也不再视为错误。
	// queue 的作用域语义保持不变：传了且非空则判定该队列的 worker，否则判定默认队列。
	queue := t.config.DefaultQueue
	if params := r.URL.Query(); len(params["queue"]) == 1 && params["queue"][0] != "" {
		queue = params["queue"][0]
	}
	tag := t.genConsumerTag(queue)

	t.healthMu.RLock()
	stat, ok := t.workerHealth[tag]
	t.healthMu.RUnlock()

	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, "no worker for consumer tag: %v", tag)
		return
	}
	if !stat.healthy(time.Now()) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w,
			"consume loop stuck: no tick within %v, %v/%v slots busy (tag: %v)",
			healthStaleThreshold,
			atomic.LoadInt64(&stat.inFlight), stat.concurrency, tag)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
