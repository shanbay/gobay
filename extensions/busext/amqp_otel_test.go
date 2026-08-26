/*
# Partition table (ECP + BVA) — busext otel tracing

# 参数/状态          | 等价类                          | 类型   | 代表值/场景                                   | 期望输出
# OTEL_ENABLE        | 未开启（默认）                  | 有效   | env 为空                                      | dispatch 不产生 span
# OTEL_ENABLE        | 开启                            | 有效   | env = "true"                                  | dispatch 产生 Consumer span / PushWithContext 注入并产生 Producer span
# delivery.Headers   | 携带合法上游 traceparent        | 有效   | "00-<32hex>-<16hex>-01"                       | Consumer span 挂到上游 trace
# handler 执行结果   | 成功                            | 有效   | Run() 返回 nil                                | span 状态非 Error
# handler 执行结果   | 失败                            | 有效   | Run() 返回 error                              | span 状态 Error，描述为 "failure"
# 发送侧 ctx         | ctx 携带活跃 span               | 有效   | 测试内手动 Start 一个 upstream span           | Producer span 与 upstream 同 TraceID，data.Headers 注入 traceparent
*/
package busext

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
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

type otelOKHandler struct{}

func (h *otelOKHandler) ParsePayload(args, kwargs []byte) error { return nil }
func (h *otelOKHandler) Run() error                             { return nil }

type otelFailHandler struct{}

func (h *otelFailHandler) ParsePayload(args, kwargs []byte) error { return nil }
func (h *otelFailHandler) Run() error                             { return errors.New("boom") }

func newOtelTestBusExt(routingKey string, handler Handler) *BusExt {
	return &BusExt{
		consumers:   map[string]Handler{routingKey: handler},
		ErrorLogger: log.New(os.Stderr, "[otel-test]", 0),
	}
}

func newOtelTestDelivery(routingKey, traceparent string) amqp.Delivery {
	headers := amqp.Table{}
	if traceparent != "" {
		headers["traceparent"] = traceparent
	}
	return amqp.Delivery{
		Headers:         headers,
		ContentType:     "application/json",
		ContentEncoding: "utf-8",
		RoutingKey:      routingKey,
		Body:            []byte("[[], {}]"),
	}
}

func TestDispatch_ConsumerSpanLinksUpstreamTrace(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	sr := newSpanRecorder(t)

	upstreamTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	upstreamSpanID := "00f067aa0ba902b7"
	b := newOtelTestBusExt("buses.test.event", &otelOKHandler{})
	delivery := newOtelTestDelivery(
		"buses.test.event",
		fmt.Sprintf("00-%s-%s-01", upstreamTraceID, upstreamSpanID),
	)

	status := b.dispatch(delivery)
	assert.Equal(t, "success", status)

	spans := sr.Ended()
	if !assert.Len(t, spans, 1) {
		return
	}
	span := spans[0]
	assert.Equal(t, "run/buses.test.event", span.Name())
	assert.Equal(t, oteltrace.SpanKindConsumer, span.SpanKind())
	assert.Equal(t, upstreamTraceID, span.SpanContext().TraceID().String())
	assert.Equal(t, upstreamSpanID, span.Parent().SpanID().String())
	assert.NotEqual(t, codes.Error, span.Status().Code)
}

func TestDispatch_HandlerFailureSetsErrorStatus(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	sr := newSpanRecorder(t)

	b := newOtelTestBusExt("buses.test.event", &otelFailHandler{})
	delivery := newOtelTestDelivery("buses.test.event", "")

	status := b.dispatch(delivery)
	assert.Equal(t, "failure", status)

	spans := sr.Ended()
	if !assert.Len(t, spans, 1) {
		return
	}
	assert.Equal(t, codes.Error, spans[0].Status().Code)
	assert.Equal(t, "failure", spans[0].Status().Description)
}

func TestDispatch_OtelDisabledByDefault_NoSpan(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "")
	sr := newSpanRecorder(t)

	b := newOtelTestBusExt("buses.test.event", &otelOKHandler{})
	status := b.dispatch(newOtelTestDelivery("buses.test.event", ""))
	assert.Equal(t, "success", status)

	assert.Empty(t, sr.Ended())
}

func TestPushWithContext_InjectsTraceparentAndProducerSpan(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	sr := newSpanRecorder(t)

	ctx, upstream := otel.Tracer("test").Start(context.Background(), "upstream")

	b := &BusExt{mocked: true}
	msg, err := BuildMsg("buses.test.event", []interface{}{}, map[string]interface{}{})
	assert.Nil(t, err)

	assert.Nil(t, b.PushWithContext(ctx, "sbay-exchange", "buses.test.event", *msg))
	upstream.End()

	tp, _ := msg.Headers["traceparent"].(string)
	assert.Contains(t, tp, upstream.SpanContext().TraceID().String())

	var producer sdktrace.ReadOnlySpan
	for _, span := range sr.Ended() {
		if span.Name() == "send/buses.test.event" {
			producer = span
		}
	}
	if !assert.NotNil(t, producer, "should record a producer span named send/buses.test.event") {
		return
	}
	assert.Equal(t, oteltrace.SpanKindProducer, producer.SpanKind())
	assert.Equal(t, upstream.SpanContext().TraceID(), producer.SpanContext().TraceID())
}
