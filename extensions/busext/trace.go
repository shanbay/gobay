package busext

import (
	"context"
	"fmt"

	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/shanbay/gobay/extensions/busext"

// HandlerWithContext 是 Handler 的可选扩展接口：实现它的 handler 在消费时会拿到
// 携带上游 trace 上下文的 ctx（消息头里的 W3C traceparent 已被提取），把 ctx 传给
// ent/redis/RPC 等下游调用即可让整条链路挂到同一个 trace 上。
// 未实现该接口的 handler 行为不变（退回 Run()）。
type HandlerWithContext interface {
	Handler
	RunWithContext(ctx context.Context) error
}

// amqpHeaderCarrier 把 amqp.Table 适配成 OTel 的 TextMapCarrier，
// 用于在 celery 协议消息头上注入/提取 traceparent（与 Python 端
// opentelemetry-instrumentation-celery 的传播方式互通）。
type amqpHeaderCarrier amqp.Table

func (c amqpHeaderCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (c amqpHeaderCarrier) Set(key, value string) {
	c[key] = value
}

func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

var _ propagation.TextMapCarrier = amqpHeaderCarrier{}

// startConsumeSpan 从消息头提取上游 trace 上下文并开启 CONSUMER span。
// OTEL_ENABLE 未开时全局 provider/propagator 均为 no-op，本函数零开销。
func startConsumeSpan(delivery amqp.Delivery) (context.Context, trace.Span) {
	ctx := context.Background()
	if delivery.Headers != nil {
		ctx = otel.GetTextMapPropagator().Extract(ctx, amqpHeaderCarrier(delivery.Headers))
	}
	return otel.Tracer(tracerName).Start(
		ctx,
		fmt.Sprintf("%s process", delivery.RoutingKey),
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", delivery.RoutingKey),
			attribute.String("messaging.operation", "process"),
		),
	)
}

// endConsumeSpan 按 dispatch 的状态字符串收尾 span（与指标的 status 口径一致）。
func endConsumeSpan(span trace.Span, status string) {
	span.SetAttributes(attribute.String("gobay.bus.status", status))
	if status != "success" {
		span.SetStatus(codes.Error, status)
	}
	span.End()
}

// injectTraceHeaders 把当前 ctx 的 trace 上下文写进待发布消息的 headers，
// Python 端 celery worker（opentelemetry-instrumentation-celery）会自动提取续链。
func injectTraceHeaders(ctx context.Context, data *amqp.Publishing) {
	if data.Headers == nil {
		data.Headers = amqp.Table{}
	}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(data.Headers))
}

// PushWithContext 在 Push 基础上：开启 PRODUCER span 并把 trace 上下文注入消息头，
// 使消费方（Go/Python 均可）续到同一条 trace。原 Push 行为不变。
func (b *BusExt) PushWithContext(ctx context.Context, exchange, routingKey string, data amqp.Publishing) error {
	ctx, span := otel.Tracer(tracerName).Start(
		ctx,
		fmt.Sprintf("%s publish", routingKey),
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", routingKey),
			attribute.String("messaging.operation", "publish"),
		),
	)
	defer span.End()
	injectTraceHeaders(ctx, &data)
	err := b.Push(exchange, routingKey, data)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// runHandler 优先走 HandlerWithContext，未实现则退回 Run()。
func runHandler(ctx context.Context, handler Handler) error {
	if h, ok := handler.(HandlerWithContext); ok {
		return h.RunWithContext(ctx)
	}
	return handler.Run()
}
