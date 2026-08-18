package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// taskDurationBuckets 与 Python coast 库 coast/celery.py 里的
// _TASK_DURATION_BUCKETS 完全一致，保证跨语言 histogram_quantile 聚合不失真
// （任何一侧单独调整都会让合并查询静默算错，必须同步改）。
//
// 档位依线上实测分布选定（单位：秒）：mall prod 与 learning staging 的样本中
// 99.8% 落在 50ms 以内，故首档下移到 5ms 以分辨「空转 / 真处理」并提供劣化
// 早期信号；中段覆盖占用 worker 的慢任务；60s 之上进 +Inf 作为告警线
// （celery 软超时 300s 会杀任务，更长的桶无实际样本）。
var taskDurationBuckets = []float64{0.005, 0.05, 0.1, 0.5, 1, 10, 60}

// AsyncTaskDurationSeconds 与 Python coast 库的 ASYNC_TASK_DURATION_SECONDS
// 同名同 label 同 buckets，可以在同一个 Prometheus/Grafana 查询里合并两边的数据。
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
// 同名同 label 同 buckets。
var BusTaskDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "bus_task_duration_seconds",
		Help:    "Time spent processing a bus message",
		Buckets: taskDurationBuckets,
	},
	[]string{"task_name", "queue", "status"},
)
