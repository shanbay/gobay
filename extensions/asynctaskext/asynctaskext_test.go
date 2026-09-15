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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	machinery "github.com/RichardKnop/machinery/v1"
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

/*
# Partition table (ECP + BVA) — step 01-watchdog-health-check
#
# 参数/状态                                    | 等价类                                 | 类型              | 代表值/场景                                                        | 期望输出 | 对应契约条目
# timeout query 参数                           | 缺失                                    | 有效（新契约）    | GET /health?queue=gobay.task.one（无 timeout）                     | 200      | MUST "timeout 缺失时不再返回 400"
# timeout query 参数                           | 存在但取值应被忽略（下边界 0）          | 有效（新契约）    | GET /health?queue=gobay.task.one&timeout=0                         | 200      | MUST "timeout/queue 保留接收但忽略"
# 心跳(lastTick) × 负载(inFlight vs concurrency)| 心跳新鲜 + 满载且队列有积压             | 有效              | concurrency=2，5 个阻塞任务（2 in-flight + 3 backlog）             | 200      | 判定式第 1 行"心跳新鲜→健康"
# 心跳(lastTick) × 负载                        | 心跳停止 + 满载（显式 concurrency）     | 有效·边界         | concurrency=2，恰好 2 个阻塞任务，worker.Quit() 后等待 > X          | 200      | 判定式第 2 行"心跳停止+满载→健康"
# 心跳(lastTick) × 负载 × concurrency 兜底      | 心跳停止 + 满载（concurrency<1 兜底）   | 有效·边界(BVA: concurrency=0) | concurrency=0 → 兜底 runtime.NumCPU()*2，恰好兜底数量个阻塞任务，Quit() 后等待 > X | 200 | MUST "concurrency<1 时必须按 runtime.NumCPU()*2 记录"
# 心跳(lastTick) × 负载                        | 心跳停止 + 有空闲槽                     | 无效（不健康路径）| concurrency=2，仅 1 个阻塞任务，worker.Quit() 后等待 > X            | 400      | 判定式第 3 行"心跳停止+有空闲槽→不健康"
#
# 说明：
# 1. X（心跳新鲜度阈值）契约固定为 60s，且与任务耗时无关。当前实现未把 X 暴露为
#    可在测试里覆盖的包级变量，为了不引用任何尚不存在的标识符（那样会导致本文件
#    无法编译，违反"今天必须能编译通过"的硬约束），本文件里凡是需要"心跳停止超过
#    X"的用例，一律真实 sleep > 60s 去触碰边界，三个相关子场景共享同一次等待。
#    可测试性建议见本次任务回复中的 testability_constraints 字段：若实现把 X 做
#    成包级 var（而非 const），未来可以在测试里临时调小 X 并 defer 还原，从而
#    避免这个 60s+ 的真实等待。
# 2. 由于全局 `:5000` 端口只允许一次 http.Handle("/health", ...) 注册
#    （healthHandlerRegistered 语义），本组新测试全部复用 TestPushConsume 里已经
#    对 taskOne 启动过 healthcheck 的那个 HTTP server，通过 StartWorker 的 queue
#    参数开新的队列/worker，而不是新建一个 AsyncTaskExt 实例。
*/

// getHealthStatus issues a GET against the shared :5000 /health endpoint and
// returns only the observable status code, closing the response body.
func getHealthStatus(t *testing.T, url string) int {
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("health request to %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// TestAsyncTaskHealth_TimeoutParamIgnored covers the "timeout query参数缺失/取值
// 无关" equivalence classes: MUST "timeout 缺失时不再返回 400" and MUST
// "timeout/queue 保留接收但忽略".
func TestAsyncTaskHealth_TimeoutParamIgnored(t *testing.T) {
	t.Run("missing timeout no longer 400", func(t *testing.T) {
		status := getHealthStatus(t, "http://127.0.0.1:5000/health?queue=gobay.task.one")
		assert.Equal(t, http.StatusOK, status,
			"missing timeout must not fail the health check under the new watchdog contract")
	})

	t.Run("timeout value is accepted but irrelevant (=0)", func(t *testing.T) {
		status := getHealthStatus(t, "http://127.0.0.1:5000/health?queue=gobay.task.one&timeout=0")
		assert.Equal(t, http.StatusOK, status,
			"timeout value must be accepted-but-ignored per contract, not used to bound a real task round-trip")
	})
}

// TestAsyncTaskHealth_FullLoadWithBacklog_Healthy covers 判定式第 1 行
// ("心跳新鲜→健康" regardless of load): a fully-loaded worker pool with a
// queued backlog must still report healthy as long as the heartbeat is
// fresh. The pre-existing implementation instead dispatches a real
// health-check task that has to wait behind the backlog for a free worker
// slot, so it times out and reports unhealthy under this exact scenario.
var blockFiveCh = make(chan struct{})

func TaskBlockFive(id int64) (int64, error) {
	<-blockFiveCh
	return id, nil
}

func TestAsyncTaskHealth_FullLoadWithBacklog_Healthy(t *testing.T) {
	if err := taskOne.RegisterWorkerHandler("blockFive", TaskBlockFive); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := taskOne.StartWorker("gobay.task_five", 2, true); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(500 * time.Millisecond) // make sure the worker is started

	t.Cleanup(func() { close(blockFiveCh) })

	// concurrency=2: send 5 tasks so 2 become in-flight (fill the pool) and
	// 3 remain queued as backlog.
	for i := 0; i < 5; i++ {
		sign := &tasks.Signature{
			Name:       "blockFive",
			RoutingKey: "gobay.task_five",
			Args:       []tasks.Arg{{Type: "int64", Value: int64(i)}},
		}
		if _, err := taskOne.SendTask(sign); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(800 * time.Millisecond) // let 2 in-flight + 3 backlog settle

	status := getHealthStatus(t, "http://127.0.0.1:5000/health?queue=gobay.task_five&timeout=5")
	assert.Equal(t, http.StatusOK, status,
		"full worker pool with a queued backlog but a fresh heartbeat must be healthy per watchdog contract row 1")
}

// TestAsyncTaskHealth_HeartbeatStopped_Group covers 判定式第 2/3 行 (heartbeat
// stopped): all three sub-scenarios share a single real sleep past X (60s)
// since X is not exposed as an overridable package-level var today (see the
// partition-table comment above), avoiding paying the 60s+ wait three times.
var (
	blockSevenCh = make(chan struct{})
	blockEightCh = make(chan struct{})
	blockNineCh  = make(chan struct{})
)

func TaskBlockSeven(id int64) (int64, error) {
	<-blockSevenCh
	return id, nil
}

func TaskBlockEight(id int64) (int64, error) {
	<-blockEightCh
	return id, nil
}

func TaskBlockNine(id int64) (int64, error) {
	<-blockNineCh
	return id, nil
}

func TestAsyncTaskHealth_HeartbeatStopped_Group(t *testing.T) {
	t.Cleanup(func() {
		close(blockSevenCh)
		close(blockEightCh)
		close(blockNineCh)
	})

	if err := taskOne.RegisterWorkerHandlers(map[string]interface{}{
		"blockSeven": TaskBlockSeven,
		"blockEight": TaskBlockEight,
		"blockNine":  TaskBlockNine,
	}); err != nil {
		t.Fatal(err)
	}

	// StartWorker 在 taskOne.lock 保护下 append 到 workers；测试在另一个
	// goroutine 里读同一个切片，必须走同一把锁，否则 -race 会报数据竞争。
	workerCount := func() int {
		taskOne.lock.Lock()
		defer taskOne.lock.Unlock()
		return len(taskOne.workers)
	}
	workerAt := func(i int) *machinery.Worker {
		taskOne.lock.Lock()
		defer taskOne.lock.Unlock()
		return taskOne.workers[i]
	}

	startIdx := workerCount()

	go func() {
		if err := taskOne.StartWorker("gobay.task_seven", 2, true); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(300 * time.Millisecond)

	go func() {
		// concurrency < 1 must be recorded as runtime.NumCPU()*2 per contract
		if err := taskOne.StartWorker("gobay.task_eight", 0, true); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(300 * time.Millisecond)

	go func() {
		if err := taskOne.StartWorker("gobay.task_nine", 2, true); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(300 * time.Millisecond)

	if n := workerCount(); n != startIdx+3 {
		t.Fatalf("expected 3 new workers to be registered on taskOne, got %d new", n-startIdx)
	}
	// All three workers below are spawned from the same taskOne AsyncTaskExt
	// instance and therefore share a single underlying broker/stop-channel
	// (see github.com/RichardKnop/machinery/v1/common.Broker.stopChan): only
	// one of them needs to be told to Quit() to freeze the heartbeat for all
	// of taskOne's active queues at once — calling Quit() on more than one
	// panics with "close of closed channel".
	workerSeven := workerAt(startIdx)

	// queue seven: fill exactly to the explicit concurrency (2) -> full, no backlog
	for i := 0; i < 2; i++ {
		sign := &tasks.Signature{Name: "blockSeven", RoutingKey: "gobay.task_seven",
			Args: []tasks.Arg{{Type: "int64", Value: int64(i)}}}
		if _, err := taskOne.SendTask(sign); err != nil {
			t.Fatal(err)
		}
	}

	// queue eight: concurrency<1 must fall back to runtime.NumCPU()*2; send
	// exactly that many blocking tasks so inFlight == the fallback value,
	// not the literal 0 that was passed in.
	fallbackConcurrency := runtime.NumCPU() * 2
	for i := 0; i < fallbackConcurrency; i++ {
		sign := &tasks.Signature{Name: "blockEight", RoutingKey: "gobay.task_eight",
			Args: []tasks.Arg{{Type: "int64", Value: int64(i)}}}
		if _, err := taskOne.SendTask(sign); err != nil {
			t.Fatal(err)
		}
	}

	// queue nine: only 1 of the 2 slots is busy -> 1 idle slot remains
	sign := &tasks.Signature{Name: "blockNine", RoutingKey: "gobay.task_nine",
		Args: []tasks.Arg{{Type: "int64", Value: int64(0)}}}
	if _, err := taskOne.SendTask(sign); err != nil {
		t.Fatal(err)
	}

	// give machinery time to actually dispatch every blocking task into a
	// worker goroutine before we freeze each worker's heartbeat via Quit()
	time.Sleep(1500 * time.Millisecond)

	// Quit() internally calls the broker's StopConsuming(), which blocks on
	// processingWG.Wait() until every in-flight task handler returns; since
	// our blocking handlers never return until the test's Cleanup closes
	// their channels, calling Quit() synchronously here would deadlock the
	// test goroutine forever. The stop signal that halts the
	// PreConsumeHandler-driven fetch loop (and therefore freezes the
	// heartbeat) is delivered essentially immediately once StopConsuming is
	// invoked, so firing Quit() in a detached goroutine is sufficient to
	// freeze the heartbeat without waiting for it to fully drain.
	go workerSeven.Quit()

	// X（心跳新鲜度阈值）在生产上是 60s，但它的语义是「多久没 tick 算卡死」，
	// 与阈值绝对值无关；实现把它声明为包级 var 正是为了让测试能压缩这段等待。
	// 覆盖成毫秒级后，三个子场景的判定路径与生产完全一致，但不必真睡 61 秒。
	restoreThreshold := healthStaleThreshold
	healthStaleThreshold = 300 * time.Millisecond
	defer func() { healthStaleThreshold = restoreThreshold }()
	time.Sleep(600 * time.Millisecond)

	t.Run("heartbeat stopped + full via explicit concurrency -> healthy", func(t *testing.T) {
		status := getHealthStatus(t, "http://127.0.0.1:5000/health?queue=gobay.task_seven&timeout=5")
		assert.Equal(t, http.StatusOK, status,
			"stale heartbeat but inFlight == explicit concurrency must be healthy per watchdog contract row 2")
	})

	t.Run("heartbeat stopped + full via concurrency<1 fallback -> healthy", func(t *testing.T) {
		status := getHealthStatus(t, "http://127.0.0.1:5000/health?queue=gobay.task_eight&timeout=5")
		assert.Equal(t, http.StatusOK, status,
			"concurrency<1 must be recorded as runtime.NumCPU()*2 so a fully-loaded pool at that size is healthy")
	})

	t.Run("heartbeat stopped + idle slot -> unhealthy", func(t *testing.T) {
		status := getHealthStatus(t, "http://127.0.0.1:5000/health?queue=gobay.task_nine&timeout=5")
		assert.Equal(t, http.StatusBadRequest, status,
			"stale heartbeat with an idle worker slot (inFlight < concurrency) must be unhealthy per watchdog contract row 3")
	})
}
