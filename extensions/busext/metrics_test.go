package busext

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

// busHistSample 读取指定 label 组合当前的 count/sum，从未 observe 过时为 (0, 0)。
func busHistSample(t *testing.T, taskName, queue, status string) (uint64, float64) {
	t.Helper()
	obs, err := busTaskDurationSeconds.GetMetricWithLabelValues(taskName, queue, status)
	assert.Nil(t, err)
	m := &dto.Metric{}
	assert.Nil(t, obs.(prometheus.Metric).Write(m))
	return m.GetHistogram().GetSampleCount(), m.GetHistogram().GetSampleSum()
}

// 成功路径：status=success，task_name 与 queue 均为 routing key，耗时被记录
func TestObserveBusTaskSuccess(t *testing.T) {
	routingKey := fmt.Sprintf("buses.metrics.ok.%d", time.Now().UnixNano())

	observeBusTask(routingKey, time.Now().Add(-10*time.Millisecond), nil)

	count, sum := busHistSample(t, routingKey, routingKey, "success")
	assert.Equal(t, uint64(1), count)
	assert.GreaterOrEqual(t, sum, 0.01)
}

// 失败路径：handler 返回 error → status=failure
func TestObserveBusTaskFailure(t *testing.T) {
	routingKey := fmt.Sprintf("buses.metrics.fail.%d", time.Now().UnixNano())

	observeBusTask(routingKey, time.Now(), errors.New("boom"))

	count, _ := busHistSample(t, routingKey, routingKey, "failure")
	assert.Equal(t, uint64(1), count)
	successCount, _ := busHistSample(t, routingKey, routingKey, "success")
	assert.Equal(t, uint64(0), successCount)
}

// 指标元数据：名称/label/bucket 与 coast 契约一致
func TestBusHistogramMetadata(t *testing.T) {
	obs, err := busTaskDurationSeconds.GetMetricWithLabelValues("meta.check", "q", "success")
	assert.Nil(t, err)
	m := &dto.Metric{}
	assert.Nil(t, obs.(prometheus.Metric).Write(m))
	assert.Equal(t, 13, len(m.GetHistogram().GetBucket()))
	assert.Equal(t, 0.05, m.GetHistogram().GetBucket()[0].GetUpperBound())
	assert.Equal(t, float64(600), m.GetHistogram().GetBucket()[12].GetUpperBound())
}
