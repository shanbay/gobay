package busext

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultMetricsServerAddr 与 Python 中间件 coast 的 Bus(9809) 保持一致，
// 方便两种语言的 worker 共用同一套 scrape 配置。
const DefaultMetricsServerAddr = ":9809"

// taskDurationBuckets 必须与 coast 的 _TASK_DURATION_BUCKETS 逐档一致，
// 否则跨语言 histogram_quantile 合并查询会得到错误的分位数。
var taskDurationBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600}

// busTaskDurationSeconds 与 coast 的同名指标做跨语言合并查询。
// label 语义对齐 coast：task_name 与 queue 均取消息的 routing key
// （celery 协议下 bus 消息的 routing key 即任务名）；status=success/failure。
var busTaskDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "bus_task_duration_seconds",
		Help:    "Time spent processing a bus message",
		Buckets: taskDurationBuckets,
	},
	[]string{"task_name", "queue", "status"},
)

// observeBusTask 记录一次 bus 消息处理的耗时与状态。
func observeBusTask(routingKey string, start time.Time, err error) {
	status := "success"
	if err != nil {
		status = "failure"
	}
	busTaskDurationSeconds.WithLabelValues(routingKey, routingKey, status).
		Observe(time.Since(start).Seconds())
}

// startMetricsServer 在消费进程内暴露 /metrics（默认 :9809）。
// 端口被占等启动失败仅记录日志，不影响消费（与 coast 行为一致）。
func (b *BusExt) startMetricsServer() {
	if b.metricsServerStarted {
		return
	}
	b.metricsServerStarted = true
	addr := b.MetricsServerAddr
	if addr == "" {
		addr = DefaultMetricsServerAddr
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			b.ErrorLogger.Printf("bus metrics server not started: %v\n", err)
		}
	}()
}
