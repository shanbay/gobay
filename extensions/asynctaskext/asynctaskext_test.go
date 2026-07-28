/*
# Partition table (ECP + BVA) — step 01-asynctask-bus-metrics (asynctaskext side)
#
# 参数/状态                | 等价类                       | 类型        | 代表值/场景                              | 期望输出                                                                          | 对应契约条目
# monitorEnabled           | 关闭（默认/未设置）          | 有效        | one_asynctask_ 在 "testing" env 下未设置 monitor_enable | 任务正常执行不受影响，不 panic，asynctask_task_duration_seconds 不含该 task_name 的记录 | 数据层 API 锚点 1
# monitorEnabled           | 开启                         | 有效        | three_asynctask_monitor_enable: true      | Init 不 panic，monitorEnabled == true                                             | 数据层 API 锚点 2/3/9
# 任务执行结果 (task outcome)| 成功                         | 有效        | TaskAddThree 正常返回                     | 指标 _count 增加、_sum > 0，status 仍为 "unknown"                                  | 数据层 API 锚点 2、行为契约"status 固定 unknown"
# 任务执行结果 (task outcome)| 返回 error                   | 有效（错误路径） | TaskFailThree 返回 error               | 指标依然被 Observe（不因失败跳过），status 仍为 "unknown"                          | 数据层 API 锚点 3
# 同类型多实例 (multi-instance)| 两个实例都开启 monitor_enable | 边界/并发场景 | three_asynctask_ + four_asynctask_ 同时 Init | 不 panic（验证包级单例、无重复 promauto 注册）                                    | 数据层 API 锚点 9、MUST "包级单例"
#
# 说明：本 step 的 asynctaskext 侧只有 2 个独立参数（monitorEnabled ×2、task outcome ×2），
# 未达到 pairwise 触发阈值（≥3 参数 × 每个 ≥2 取值），因此未调用 pairwise 脚本，
# 采用穷举的 2×2 组合（关闭+成功 已由锚点1覆盖；开启+成功、开启+失败 由锚点2/3覆盖）。
*/
package asynctaskext

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RichardKnop/machinery/v1/backends/result"
	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"

	"github.com/shanbay/gobay"
)

var (
	taskOne AsyncTaskExt
	taskTwo AsyncTaskExt
)

func init() {
	taskOne = AsyncTaskExt{NS: "one_asynctask_"}
	taskTwo = AsyncTaskExt{NS: "two_asynctask_"}

	app, _ := gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"oneasynctask": &taskOne,
			"twoasynctask": &taskTwo,
		},
	)
	if err := app.Init(); err != nil {
		log.Panic(err)
	}
}

func TaskAdd(args ...int64) (int64, error) {
	sum := int64(0)
	for _, arg := range args {
		sum += arg
	}
	return sum, nil
}

func TaskSub(arg1, arg2 int64) (int64, error) {
	return arg1 - arg2, nil
}

func TaskSubWithContext(ctx context.Context, arg1, arg2 int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return arg1 - arg2, nil
}

// TaskAddThree / TaskFailThree are used exclusively by the monitor tests below,
// with names that never collide with "add"/"sub"/"subCtx" registered on taskOne,
// so metric assertions can't accidentally match unrelated task executions.
func TaskAddThree(args ...int64) (int64, error) {
	sum := int64(0)
	for _, arg := range args {
		sum += arg
	}
	return sum, nil
}

func TaskFailThree(arg int64) (int64, error) {
	return 0, errors.New("intentional failure for asynctaskext monitor test")
}

var (
	taskThree AsyncTaskExt
	taskFour  AsyncTaskExt

	asyncTaskMetricsOnce sync.Once
)

const asyncTaskMetricsAddr = "127.0.0.1:2113"

// startAsyncTaskMetricsServer exposes prometheus.DefaultRegisterer via
// promhttp.Handler(), mirroring extensions/cachext's TestCacheExt_Cached_Monitor
// pattern (config-gated instrumentation + no self-built /metrics server in
// production code, only in the test harness).
func startAsyncTaskMetricsServer() {
	asyncTaskMetricsOnce.Do(func() {
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(asyncTaskMetricsAddr, nil); err != nil {
				log.Fatalf("error when start prometheus server: %v\n", err)
			}
		}()
		time.Sleep(200 * time.Millisecond)
	})
}

func fetchAsyncTaskMetrics(t *testing.T) string {
	resp, err := http.Get("http://" + asyncTaskMetricsAddr + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// extractMetricValue looks up "<family>{<labels>} <value>" in the exposition
// text and parses the numeric value, so tests can assert e.g. "_sum > 0"
// instead of only checking for substring presence.
func extractMetricValue(t *testing.T, metrics, family, labels string) float64 {
	prefix := family + "{" + labels + "} "
	idx := strings.Index(metrics, prefix)
	if idx == -1 {
		t.Fatalf("metric not found: %s", prefix)
	}
	rest := metrics[idx+len(prefix):]
	end := strings.IndexByte(rest, '\n')
	if end == -1 {
		end = len(rest)
	}
	valueStr := strings.TrimSpace(rest[:end])
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		t.Fatalf("failed to parse metric value %q: %v", valueStr, err)
	}
	return value
}

func TestPushConsume(t *testing.T) {
	if err := taskOne.RegisterWorkerHandlers(map[string]interface{}{
		"add": TaskAdd, "sub": TaskSub, "subCtx": TaskSubWithContext,
	}); err != nil {
		t.Error(err)
	}
	go func() {
		// use default queue
		if err := taskOne.StartWorker("", 1, true); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(500 * time.Millisecond) // Make sure the worker is started
	go func() {
		if err := taskOne.StartWorker("gobay.task_sub", 1, true); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(500 * time.Millisecond) // Make sure the workers is started

	// health check
	resp, err := http.Get("http://127.0.0.1:5000/health?timeout=5&queue=gobay.task_sub")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("%v %s", resp, err)
	}
	resp, err = http.Get("http://127.0.0.1:5000/health?timeout=5&queue=")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("%v %s", resp, err)
	}
	resp, err = http.Get("http://127.0.0.1:5000/health?timeout=5")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("%v %s", resp, err)
	}
	resp, err = http.Get("http://127.0.0.1:5000/health?timeout=5&queue=nosuchqueue")
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Errorf("%v %s", resp, err)
	}

	signs := []*tasks.Signature{
		{
			Name: "add",
			Args: []tasks.Arg{ // use default queue
				{
					Type:  "int64",
					Value: 1,
				},
				{
					Type:  "int64",
					Value: 2,
				},
				{
					Type:  "int64",
					Value: 3,
				},
			},
		},
		{
			Name:       "sub",
			RoutingKey: "gobay.task_sub",
			Args: []tasks.Arg{
				{
					Type:  "int64",
					Value: 7,
				},
				{
					Type:  "int64",
					Value: 1,
				},
			},
		},
		{
			Name:       "subCtx",
			RoutingKey: "gobay.task_sub",
			Args: []tasks.Arg{
				{
					Type:  "int64",
					Value: 10,
				},
				{
					Type:  "int64",
					Value: 4,
				},
			},
		},
	}
	for _, sign := range signs {
		var (
			asyncResult *result.AsyncResult
			err         error
		)
		if sign.Name == "subCtx" {
			asyncResult, err = taskOne.SendTaskWithContext(context.Background(), sign)
		} else {
			asyncResult, err = taskOne.SendTask(sign)
		}
		if err != nil {
			t.Error(err)
		} else if results, err := asyncResult.Get(time.Millisecond * 5); err != nil {
			t.Error(err)
		} else if res, ok := results[0].Interface().(int64); !ok || res != 6 {
			t.Error("result error")
		}
	}
}

func TestMultiTaskExtStartWorker(t *testing.T) {
	t.Run("1: 第一个 task StartWorker, 允许 healthcheck, 正常", func(t *testing.T) {
		go func() {
			// use default queue
			if err := taskOne.StartWorker("", 1, true); err != nil {
				t.Error(err)
			}
		}()
	})

	t.Run("2: 第二个 task StartWorker, 不允许 healthcheck, 正常运行", func(t *testing.T) {
		go func() {
			if err := taskTwo.StartWorker("", 1, false); err != nil {
				t.Error(err)
			}
		}()
	})

	t.Run("3: 第二个 task StartWorker, 允许 healthcheck, 会 panic", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = taskTwo.StartWorker("", 1, true)
		})
	})
}

// TestAsyncTaskExt_Monitor_Disabled 覆盖锚点 1：monitor_enable 默认关闭时，
// monitorEnabled 保持 false，任务正常执行不受影响、不 panic，且不产生任何
// asynctask_task_duration_seconds 记录。
func TestAsyncTaskExt_Monitor_Disabled(t *testing.T) {
	startAsyncTaskMetricsServer()

	assert.False(t, taskOne.monitorEnabled)

	sign := &tasks.Signature{
		Name: "add",
		Args: []tasks.Arg{
			{Type: "int64", Value: 5},
			{Type: "int64", Value: 6},
		},
	}
	asyncResult, err := taskOne.SendTask(sign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := asyncResult.Get(5 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	data := fetchAsyncTaskMetrics(t)
	assert.NotContains(t, data,
		`asynctask_task_duration_seconds_count{queue="gobay.task.one",status="unknown",task_name="add"}`)
}

// TestAsyncTaskExt_Monitor covers 锚点 2/3/9：monitor_enable=true 下，
// 两个同类型实例（three_asynctask_/four_asynctask_）各自 Init 不 panic，
// 成功任务和失败任务都会被 Observe，status 始终固定 "unknown"。
func TestAsyncTaskExt_Monitor(t *testing.T) {
	startAsyncTaskMetricsServer()

	taskThree = AsyncTaskExt{NS: "three_asynctask_"}
	taskFour = AsyncTaskExt{NS: "four_asynctask_"}

	t.Run("9: 同类型两个实例都开启 monitor_enable 各自 Init 不 panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			app, err := gobay.CreateApp(
				"../../testdata",
				"asynctaskmonitored",
				map[gobay.Key]gobay.Extension{
					"taskthree": &taskThree,
					"taskfour":  &taskFour,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := app.Init(); err != nil {
				t.Fatal(err)
			}
		})
		assert.True(t, taskThree.monitorEnabled)
		assert.True(t, taskFour.monitorEnabled)
	})

	if err := taskThree.RegisterWorkerHandlers(map[string]interface{}{
		"addThree":  TaskAddThree,
		"failThree": TaskFailThree,
	}); err != nil {
		t.Fatal(err)
	}
	go func() {
		// use default queue "gobay.task.three"
		if err := taskThree.StartWorker("", 1, false); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(500 * time.Millisecond) // make sure the worker is started

	t.Run("2: monitor_enable=true 下执行已注册任务成功 -> _count 增加、_sum > 0", func(t *testing.T) {
		sign := &tasks.Signature{
			Name: "addThree",
			Args: []tasks.Arg{
				{Type: "int64", Value: 3},
				{Type: "int64", Value: 4},
			},
		}
		asyncResult, err := taskThree.SendTask(sign)
		if err != nil {
			t.Fatal(err)
		}
		if results, err := asyncResult.Get(5 * time.Millisecond); err != nil {
			t.Fatal(err)
		} else if res, ok := results[0].Interface().(int64); !ok || res != 7 {
			t.Fatalf("unexpected task result: %v", results)
		}
		time.Sleep(300 * time.Millisecond)

		data := fetchAsyncTaskMetrics(t)
		labels := `queue="gobay.task.three",status="unknown",task_name="addThree"`
		assert.Contains(t, data, `asynctask_task_duration_seconds_count{`+labels+`} 1`)
		sum := extractMetricValue(t, data, "asynctask_task_duration_seconds_sum", labels)
		assert.Greater(t, sum, 0.0)
	})

	t.Run("3: monitor_enable=true 下执行返回 error 的任务仍被 Observe，status 仍为 unknown", func(t *testing.T) {
		sign := &tasks.Signature{
			Name: "failThree",
			Args: []tasks.Arg{
				{Type: "int64", Value: 1},
			},
		}
		asyncResult, err := taskThree.SendTask(sign)
		if err != nil {
			t.Fatal(err)
		}
		// TaskFailThree itself returns an error; we only care whether the
		// metric was recorded regardless of the task outcome, matching the
		// "不因失败而跳过" 行为契约.
		_, _ = asyncResult.Get(5 * time.Millisecond)
		time.Sleep(300 * time.Millisecond)

		data := fetchAsyncTaskMetrics(t)
		labels := `queue="gobay.task.three",status="unknown",task_name="failThree"`
		assert.Contains(t, data, `asynctask_task_duration_seconds_count{`+labels+`} 1`)
	})
}
