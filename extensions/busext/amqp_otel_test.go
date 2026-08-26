/*
# Partition table (ECP + BVA) — busext otel tracing

# 参数/状态          | 等价类                          | 类型   | 代表值/场景                                   | 期望输出
# OTEL_ENABLE        | 未开启（默认）                  | 有效   | env 为空                                      | dispatch 不产生 span
# OTEL_ENABLE        | 开启                            | 有效   | env = "true"                                  | dispatch 产生 Consumer span / PushWithContext 注入并产生 Producer span
# delivery.Headers   | 携带合法上游 traceparent        | 有效   | "00-<32hex>-<16hex>-01"                       | Consumer span 挂到上游 trace
# handler 执行结果   | 成功                            | 有效   | Run() 返回 nil                                | span 状态非 Error
# handler 执行结果   | 失败                            | 有效   | Run() 返回 error                              | span 状态 Error，描述为 "failure"
# 发送侧 ctx         | ctx 携带活跃 span               | 有效   | 测试内手动 Start 一个 upstream span           | Producer span 与 upstream 同 TraceID，data.Headers 注入 traceparent
# handler 接口形态   | 实现 HandlerWithContext         | 有效   | ctxRecordingHandler                           | 走 RunWithContext（不走 Run），ctx 携带上游 TraceID
# handler 接口形态   | 实现 HandlerWithContext+otel 关 | 边界   | env 为空 + ctxRecordingHandler                | 仍走 RunWithContext，ctx 为 Background，零 span
# span 属性          | Consumer/Producer 语义属性      | 有效   | messaging.system/destination.name/operation   | 属性齐全，gobay.bus.status 记录结果状态
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

type ctxRecordingHandler struct {
	gotCtx     context.Context
	ranWithCtx bool
	ranPlain   bool
}

func (h *ctxRecordingHandler) ParsePayload(args, kwargs []byte) error { return nil }
func (h *ctxRecordingHandler) Run() error                             { h.ranPlain = true; return nil }
func (h *ctxRecordingHandler) RunWithContext(ctx context.Context) error {
	h.ranWithCtx = true
	h.gotCtx = ctx
	return nil
}

func TestDispatch_HandlerWithContext_ReceivesUpstreamTraceContext(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	newSpanRecorder(t)

	upstreamTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	h := &ctxRecordingHandler{}
	b := newOtelTestBusExt("buses.test.event", h)
	delivery := newOtelTestDelivery(
		"buses.test.event",
		fmt.Sprintf("00-%s-00f067aa0ba902b7-01", upstreamTraceID),
	)

	assert.Equal(t, "success", b.dispatch(delivery))
	assert.True(t, h.ranWithCtx, "should call RunWithContext instead of Run")
	assert.False(t, h.ranPlain)
	if !assert.NotNil(t, h.gotCtx) {
		return
	}
	assert.Equal(t, upstreamTraceID,
		oteltrace.SpanContextFromContext(h.gotCtx).TraceID().String())
}

func TestDispatch_HandlerWithContext_WorksWithOtelDisabled(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "")
	sr := newSpanRecorder(t)

	h := &ctxRecordingHandler{}
	b := newOtelTestBusExt("buses.test.event", h)

	assert.Equal(t, "success", b.dispatch(newOtelTestDelivery("buses.test.event", "")))
	assert.True(t, h.ranWithCtx, "interface dispatch must not depend on OTEL_ENABLE")
	assert.False(t, h.ranPlain)
	assert.NotNil(t, h.gotCtx)
	assert.Empty(t, sr.Ended())
}

func spanAttr(span sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	return ""
}

func TestDispatch_ConsumerSpanHasMessagingAttributes(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	sr := newSpanRecorder(t)

	b := newOtelTestBusExt("buses.test.event", &otelOKHandler{})
	assert.Equal(t, "success", b.dispatch(newOtelTestDelivery("buses.test.event", "")))

	spans := sr.Ended()
	if !assert.Len(t, spans, 1) {
		return
	}
	span := spans[0]
	assert.Equal(t, "rabbitmq", spanAttr(span, "messaging.system"))
	assert.Equal(t, "buses.test.event", spanAttr(span, "messaging.destination.name"))
	assert.Equal(t, "process", spanAttr(span, "messaging.operation"))
	assert.Equal(t, "success", spanAttr(span, "gobay.bus.status"))
}

func TestPushWithContext_ProducerSpanHasMessagingAttributes(t *testing.T) {
	t.Setenv("OTEL_ENABLE", "true")
	sr := newSpanRecorder(t)

	b := &BusExt{mocked: true}
	msg, err := BuildMsg("buses.test.event", []interface{}{}, map[string]interface{}{})
	assert.Nil(t, err)
	assert.Nil(t, b.PushWithContext(context.Background(), "sbay-exchange", "buses.test.event", *msg))

	spans := sr.Ended()
	if !assert.Len(t, spans, 1) {
		return
	}
	span := spans[0]
	assert.Equal(t, "rabbitmq", spanAttr(span, "messaging.system"))
	assert.Equal(t, "buses.test.event", spanAttr(span, "messaging.destination.name"))
	assert.Equal(t, "publish", spanAttr(span, "messaging.operation"))
}
