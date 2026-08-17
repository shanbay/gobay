package busext

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func newTestSpanContext() trace.SpanContext {
	traceID, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	spanID, _ := trace.SpanIDFromHex("b7ad6b7169203331")
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
}

// 注入：ctx 的 trace 上下文写进消息头，traceparent 出现且携带 trace id
func TestInjectTraceHeaders(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	ctx := trace.ContextWithSpanContext(context.Background(), newTestSpanContext())

	data := amqp.Publishing{}
	injectTraceHeaders(ctx, &data)

	tp, ok := data.Headers["traceparent"].(string)
	assert.True(t, ok)
	assert.Contains(t, tp, "0af7651916cd43dd8448eb211c80319c")
}

// 提取：带 traceparent 的消息头 Extract 后能还原同一个 trace id（与注入互逆）
func TestExtractRoundTrip(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	ctx := trace.ContextWithSpanContext(context.Background(), newTestSpanContext())
	data := amqp.Publishing{}
	injectTraceHeaders(ctx, &data)

	extracted := otel.GetTextMapPropagator().Extract(context.Background(), amqpHeaderCarrier(data.Headers))
	sc := trace.SpanContextFromContext(extracted)
	assert.True(t, sc.IsValid())
	assert.Equal(t, "0af7651916cd43dd8448eb211c80319c", sc.TraceID().String())
	assert.True(t, sc.IsSampled())
}

// 非 string 值与缺失键不 panic，返回空
func TestAmqpHeaderCarrierGetNonString(t *testing.T) {
	c := amqpHeaderCarrier(amqp.Table{"retries": 0, "id": "abc"})
	assert.Equal(t, "", c.Get("retries"))
	assert.Equal(t, "", c.Get("missing"))
	assert.Equal(t, "abc", c.Get("id"))
}

type ctxRecordingHandler struct {
	gotCtx context.Context
	ran    bool
}

func (h *ctxRecordingHandler) ParsePayload(args, kwargs []byte) error { return nil }
func (h *ctxRecordingHandler) Run() error                             { h.ran = true; return nil }
func (h *ctxRecordingHandler) RunWithContext(ctx context.Context) error {
	h.gotCtx = ctx
	h.ran = true
	return nil
}

type plainHandler struct{ ran bool }

func (h *plainHandler) ParsePayload(args, kwargs []byte) error { return nil }
func (h *plainHandler) Run() error                             { h.ran = true; return nil }

func testBusExt(routingKey string, handler Handler) *BusExt {
	return &BusExt{
		consumers:   map[string]Handler{routingKey: handler},
		ErrorLogger: log.New(os.Stderr, "", 0),
	}
}

func testDelivery(routingKey string) amqp.Delivery {
	body, _ := json.Marshal([]interface{}{[]interface{}{}, map[string]interface{}{}, map[string]interface{}{}})
	return amqp.Delivery{
		RoutingKey:      routingKey,
		ContentType:     "application/json",
		ContentEncoding: "utf-8",
		Headers:         amqp.Table{"id": "test"},
		Body:            body,
	}
}

// dispatch：实现 HandlerWithContext 的 handler 收到携带上游 trace 的 ctx
func TestDispatchPassesTraceContextToHandler(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	h := &ctxRecordingHandler{}
	b := testBusExt("buses.trace.ctx", h)

	ctx := trace.ContextWithSpanContext(context.Background(), newTestSpanContext())
	status := b.dispatch(ctx, testDelivery("buses.trace.ctx"))

	assert.Equal(t, "success", status)
	assert.True(t, h.ran)
	assert.NotNil(t, h.gotCtx)
	assert.Equal(t, "0af7651916cd43dd8448eb211c80319c",
		trace.SpanContextFromContext(h.gotCtx).TraceID().String())
}

// dispatch：未实现扩展接口的 handler 走原 Run()，行为不变
func TestDispatchFallsBackToRun(t *testing.T) {
	h := &plainHandler{}
	b := testBusExt("buses.trace.plain", h)

	status := b.dispatch(context.Background(), testDelivery("buses.trace.plain"))

	assert.Equal(t, "success", status)
	assert.True(t, h.ran)
}

// startConsumeSpan：从消息头提取 traceparent 后，span 的 ctx 归属同一 trace
func TestStartConsumeSpanExtractsRemoteContext(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	d := testDelivery("buses.trace.span")
	d.Headers["traceparent"] = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	ctx, span := startConsumeSpan(d)
	defer span.End()

	assert.Equal(t, "0af7651916cd43dd8448eb211c80319c",
		trace.SpanContextFromContext(ctx).TraceID().String())
}
