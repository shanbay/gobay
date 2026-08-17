package asynctaskext

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	machineryConfig "github.com/RichardKnop/machinery/v1/config"
	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

// asyncTaskHistSample 读取指定 label 组合当前的 count/sum，从未 observe 过时为 (0, 0)。
func asyncTaskHistSample(t *testing.T, taskName, queue, status string) (uint64, float64) {
	t.Helper()
	obs, err := asyncTaskDurationSeconds.GetMetricWithLabelValues(taskName, queue, status)
	assert.Nil(t, err)
	m := &dto.Metric{}
	assert.Nil(t, obs.(prometheus.Metric).Write(m))
	return m.GetHistogram().GetSampleCount(), m.GetHistogram().GetSampleSum()
}

func metricsTestExt() *AsyncTaskExt {
	return &AsyncTaskExt{
		NS:     "asynctask_",
		config: &machineryConfig.Config{DefaultQueue: "gobay.task"},
	}
}

// 无 ctx handler：queue 回退到 DefaultQueue；nil error → success；耗时被记录
func TestWrapTaskHandlerSuccessWithDefaultQueue(t *testing.T) {
	ext := metricsTestExt()
	name := fmt.Sprintf("metrics.ok.%d", time.Now().UnixNano())

	wrapped := ext.wrapTaskHandler(name, func() error {
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	task, err := tasks.New(wrapped, []tasks.Arg{})
	assert.Nil(t, err)
	_, err = task.Call()
	assert.Nil(t, err)

	count, sum := asyncTaskHistSample(t, name, "gobay.task", "success")
	assert.Equal(t, uint64(1), count)
	assert.GreaterOrEqual(t, sum, 0.005)
}

// ctx handler：queue 取 signature 的 RoutingKey
func TestWrapTaskHandlerQueueFromSignatureRoutingKey(t *testing.T) {
	ext := metricsTestExt()
	name := fmt.Sprintf("metrics.ctx.%d", time.Now().UnixNano())

	wrapped := ext.wrapTaskHandler(name, func(ctx context.Context) error {
		return nil
	})
	sig := &tasks.Signature{Name: name, RoutingKey: "gobay.task_sub"}
	task, err := tasks.NewWithSignature(wrapped, sig)
	assert.Nil(t, err)
	_, err = task.Call()
	assert.Nil(t, err)

	count, _ := asyncTaskHistSample(t, name, "gobay.task_sub", "success")
	assert.Equal(t, uint64(1), count)
	// DefaultQueue 组合不应被记录
	fallbackCount, _ := asyncTaskHistSample(t, name, "gobay.task", "success")
	assert.Equal(t, uint64(0), fallbackCount)
}

// handler 返回普通 error → status=failure
func TestWrapTaskHandlerFailure(t *testing.T) {
	ext := metricsTestExt()
	name := fmt.Sprintf("metrics.fail.%d", time.Now().UnixNano())

	wrapped := ext.wrapTaskHandler(name, func() error {
		return errors.New("boom")
	})
	task, err := tasks.New(wrapped, []tasks.Arg{})
	assert.Nil(t, err)
	_, err = task.Call()
	assert.NotNil(t, err)

	count, _ := asyncTaskHistSample(t, name, "gobay.task", "failure")
	assert.Equal(t, uint64(1), count)
}

// handler 返回 ErrRetryTaskLater → status=retry（对齐 celery RETRY state）
func TestWrapTaskHandlerRetry(t *testing.T) {
	ext := metricsTestExt()
	name := fmt.Sprintf("metrics.retry.%d", time.Now().UnixNano())

	wrapped := ext.wrapTaskHandler(name, func() error {
		return tasks.NewErrRetryTaskLater("try again", time.Second)
	})
	task, err := tasks.New(wrapped, []tasks.Arg{})
	assert.Nil(t, err)
	_, _ = task.Call()

	count, _ := asyncTaskHistSample(t, name, "gobay.task", "retry")
	assert.Equal(t, uint64(1), count)
	failureCount, _ := asyncTaskHistSample(t, name, "gobay.task", "failure")
	assert.Equal(t, uint64(0), failureCount)
}

// 包装不改变 handler 的参数与返回值传递
func TestWrapTaskHandlerPreservesArgsAndResults(t *testing.T) {
	ext := metricsTestExt()
	name := fmt.Sprintf("metrics.args.%d", time.Now().UnixNano())

	wrapped := ext.wrapTaskHandler(name, func(a, b int64) (int64, error) {
		return a + b, nil
	})
	task, err := tasks.New(wrapped, []tasks.Arg{
		{Type: "int64", Value: int64(1)},
		{Type: "int64", Value: int64(2)},
	})
	assert.Nil(t, err)
	results, err := task.Call()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "3", fmt.Sprintf("%v", results[0].Value))
}

// 变参 handler（TaskAdd 形态）：MakeFunc 收集的变参必须经 CallSlice 正确还原
func TestWrapTaskHandlerVariadic(t *testing.T) {
	ext := metricsTestExt()
	name := fmt.Sprintf("metrics.variadic.%d", time.Now().UnixNano())

	wrapped := ext.wrapTaskHandler(name, func(args ...int64) (int64, error) {
		sum := int64(0)
		for _, arg := range args {
			sum += arg
		}
		return sum, nil
	})
	task, err := tasks.New(wrapped, []tasks.Arg{
		{Type: "int64", Value: int64(1)},
		{Type: "int64", Value: int64(2)},
		{Type: "int64", Value: int64(3)},
	})
	assert.Nil(t, err)
	results, err := task.Call()
	assert.Nil(t, err)
	assert.Equal(t, "6", fmt.Sprintf("%v", results[0].Value))

	count, _ := asyncTaskHistSample(t, name, "gobay.task", "success")
	assert.Equal(t, uint64(1), count)
}

// 指标元数据：名称/label/bucket 与 coast 契约一致
func TestAsyncTaskHistogramMetadata(t *testing.T) {
	obs, err := asyncTaskDurationSeconds.GetMetricWithLabelValues("meta.check", "q", "success")
	assert.Nil(t, err)
	m := &dto.Metric{}
	assert.Nil(t, obs.(prometheus.Metric).Write(m))
	// 13 档显式 bucket（+Inf 不出现在 dto 中）
	assert.Equal(t, 13, len(m.GetHistogram().GetBucket()))
	assert.Equal(t, 0.05, m.GetHistogram().GetBucket()[0].GetUpperBound())
	assert.Equal(t, float64(600), m.GetHistogram().GetBucket()[12].GetUpperBound())
}
