package asynctaskext

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/RichardKnop/machinery/v1/log"
	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultMetricsServerAddr 与 Python 中间件 coast 的 AsyncTask(9808) 保持一致，
// 方便两种语言的 worker 共用同一套 scrape 配置。
const DefaultMetricsServerAddr = ":9808"

// taskDurationBuckets 必须与 coast 的 _TASK_DURATION_BUCKETS 逐档一致，
// 否则跨语言 histogram_quantile 合并查询会得到错误的分位数。
var taskDurationBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600}

// asyncTaskDurationSeconds 与 coast 的同名指标做跨语言合并查询。
// label 语义对齐 coast：task_name=注册名；queue=signature 的 RoutingKey（≈消费队列）；
// status=success/failure/retry。
var asyncTaskDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "asynctask_task_duration_seconds",
		Help:    "Time spent processing an asynctask message",
		Buckets: taskDurationBuckets,
	},
	[]string{"task_name", "queue", "status"},
)

var ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()

// wrapTaskHandler 用反射包装业务 handler，记录执行耗时与状态，保持函数签名不变。
// handler 首参为 context.Context 时从 signature 取 RoutingKey 作为 queue label，
// 否则回退到配置的 DefaultQueue。
func (t *AsyncTaskExt) wrapTaskHandler(name string, handler interface{}) interface{} {
	hv := reflect.ValueOf(handler)
	ht := hv.Type()
	if ht.Kind() != reflect.Func {
		// 非法 handler 原样返回，由 machinery 的注册校验负责报错
		return handler
	}
	hasCtx := ht.NumIn() > 0 && ht.In(0) == ctxType
	isVariadic := ht.IsVariadic()
	return reflect.MakeFunc(ht, func(args []reflect.Value) []reflect.Value {
		queue := ""
		if t.config != nil {
			queue = t.config.DefaultQueue
		}
		if hasCtx && len(args) > 0 {
			if ctx, ok := args[0].Interface().(context.Context); ok {
				if sig := tasks.SignatureFromContext(ctx); sig != nil && sig.RoutingKey != "" {
					queue = sig.RoutingKey
				}
			}
		}
		if queue == "" {
			queue = "unknown"
		}
		start := time.Now()
		// MakeFunc 的内层函数收到的变参已被收集为 slice，回调原函数必须用 CallSlice
		var results []reflect.Value
		if isVariadic {
			results = hv.CallSlice(args)
		} else {
			results = hv.Call(args)
		}
		asyncTaskDurationSeconds.WithLabelValues(name, queue, taskStatus(results)).
			Observe(time.Since(start).Seconds())
		return results
	}).Interface()
}

// taskStatus 从 handler 返回值判断任务状态。machinery 约定最后一个返回值为 error：
// nil → success；ErrRetryTaskLater → retry（对齐 celery 的 RETRY state）；其余 → failure。
func taskStatus(results []reflect.Value) string {
	n := len(results)
	if n == 0 {
		return "success"
	}
	last := results[n-1]
	if last.Kind() != reflect.Interface || last.IsNil() {
		return "success"
	}
	err, ok := last.Interface().(error)
	if !ok || err == nil {
		return "success"
	}
	var retryErr tasks.ErrRetryTaskLater
	if errors.As(err, &retryErr) {
		return "retry"
	}
	return "failure"
}

// startMetricsServer 在 worker 进程内暴露 /metrics（默认 :9808）。
// 端口被占等启动失败仅记录日志，不影响 worker 消费（与 coast 行为一致）。
// 调用方需持有 t.lock。
func (t *AsyncTaskExt) startMetricsServer() {
	if t.metricsServerStarted {
		return
	}
	t.metricsServerStarted = true
	addr := t.MetricsServerAddr
	if addr == "" {
		addr = DefaultMetricsServerAddr
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.WARNING.Printf("asynctask metrics server not started: %v\n", err)
		}
	}()
}
