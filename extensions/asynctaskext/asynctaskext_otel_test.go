/*
# Partition table (ECP + BVA) — asynctaskext otel tracing

# 参数/状态          | 等价类                          | 类型   | 代表值/场景                                   | 期望输出
# OTEL_ENABLE        | 未开启（默认）                  | 有效   | env 为空                                      | otelTaskStart/otelTaskEnd 不产生 span，SendTaskWithContext 不注入 header
# OTEL_ENABLE        | 开启                            | 有效   | env = "true"                                  | 产生 span / 注入 traceparent
# 消息头 traceparent | 携带合法上游 traceparent        | 有效   | "00-<32hex>-<16hex>-01"                       | Consumer span 挂到上游 trace（TraceID 相同、parent 为上游 span）
# 消息头 traceparent | 无（Headers 为空/缺 key）       | 边界   | sig.Headers == nil                            | 自建 root Consumer span，TraceID 合法
# 发送侧 ctx         | ctx 携带活跃 span               | 有效   | 测试内手动 Start 一个 upstream span           | Producer span 与 upstream 同 TraceID，sig.Headers 注入 traceparent
*/
package asynctaskext

import (
	"context"
	"fmt"
	"testing"

	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// newSpanRecorder 把全局 TracerProvider 换成带 recorder 的 provider，测试结束后恢复。
func newSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

func TestOtelTaskSpan_LinksUpstreamTrace(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	sr := newSpanRecorder(t)

	upstreamTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	upstreamSpanID := "00f067aa0ba902b7"
	sig := &tasks.Signature{
		UUID: "otel-task-uuid-1",
		Name: "TaskAdd",
		Headers: tasks.Headers{
			"traceparent": fmt.Sprintf("00-%s-%s-01", upstreamTraceID, upstreamSpanID),
		},
	}

	taskOne.otelTaskStart(sig)
	taskOne.otelTaskEnd(sig)

	spans := sr.Ended()
	if !assert.Len(t, spans, 1) {
		return
	}
	span := spans[0]
	assert.Equal(t, "run/TaskAdd", span.Name())
	assert.Equal(t, oteltrace.SpanKindConsumer, span.SpanKind())
	assert.Equal(t, upstreamTraceID, span.SpanContext().TraceID().String())
	assert.Equal(t, upstreamSpanID, span.Parent().SpanID().String())
}

func TestOtelTaskSpan_NoUpstreamHeaders_StartsNewRootTrace(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	sr := newSpanRecorder(t)

	sig := &tasks.Signature{UUID: "otel-task-uuid-2", Name: "TaskAdd"}

	taskOne.otelTaskStart(sig)
	taskOne.otelTaskEnd(sig)

	spans := sr.Ended()
	if !assert.Len(t, spans, 1) {
		return
	}
	span := spans[0]
	assert.Equal(t, oteltrace.SpanKindConsumer, span.SpanKind())
	assert.True(t, span.SpanContext().TraceID().IsValid())
	assert.False(t, span.Parent().IsValid())
}

func TestOtelTaskSpan_DisabledByDefault_NoSpan(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "")
	sr := newSpanRecorder(t)

	sig := &tasks.Signature{UUID: "otel-task-uuid-3", Name: "TaskAdd"}

	taskOne.otelTaskStart(sig)
	taskOne.otelTaskEnd(sig)

	assert.Empty(t, sr.Ended())
}

func TestSendTaskWithContext_InjectsTraceparentAndProducerSpan(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	sr := newSpanRecorder(t)

	ctx, upstream := otel.Tracer("test").Start(context.Background(), "upstream")

	sig := &tasks.Signature{
		Name: "TaskAdd",
		Args: []tasks.Arg{
			{Type: "int64", Value: 1},
			{Type: "int64", Value: 2},
		},
	}
	_, err := taskOne.SendTaskWithContext(ctx, sig)
	assert.Nil(t, err)
	upstream.End()

	tp, _ := sig.Headers["traceparent"].(string)
	assert.Contains(t, tp, upstream.SpanContext().TraceID().String())

	var producer sdktrace.ReadOnlySpan
	for _, span := range sr.Ended() {
		if span.Name() == "send/TaskAdd" {
			producer = span
		}
	}
	if !assert.NotNil(t, producer, "should record a producer span named send/TaskAdd") {
		return
	}
	assert.Equal(t, oteltrace.SpanKindProducer, producer.SpanKind())
	assert.Equal(t, upstream.SpanContext().TraceID(), producer.SpanContext().TraceID())
}
