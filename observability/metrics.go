package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// taskDurationBuckets 覆盖毫秒级 bus 消息到分钟级 asynctask 长任务：
// 5ms 起步足以分辨轻量消息，60s 封顶兜住慢任务，只留 7 档以压低
// histogram 产生的时间序列数量。
var taskDurationBuckets = []float64{0.005, 0.05, 0.1, 0.5, 1, 10, 60}

// AsyncTaskDurationSeconds 与 Python coast 库的 ASYNC_TASK_DURATION_SECONDS
// 同名同 label，可以在同一个 Prometheus/Grafana 查询里合并两边的数据；
// 两边 buckets 不同，跨语言做 histogram_quantile 时需要留意分位数偏差。
// 包级 var，Go 保证进程内只初始化一次——即便同一个 extension 类型有多个实例
// （如 one_asynctask_/two_asynctask_）反复 Init 也不会重复 promauto 注册导致 panic。
var AsyncTaskDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "asynctask_task_duration_seconds",
		Help:    "Time spent processing an asynctask message",
		Buckets: taskDurationBuckets,
	},
	[]string{"task_name", "queue", "status"},
)

// BusTaskDurationSeconds 与 Python coast 库的 BUS_TASK_DURATION_SECONDS
// 同名同 label。
var BusTaskDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "bus_task_duration_seconds",
		Help:    "Time spent processing a bus message",
		Buckets: taskDurationBuckets,
	},
	[]string{"task_name", "queue", "status"},
)
