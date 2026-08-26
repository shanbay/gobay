package busext

import (
	"context"

	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/shanbay/gobay/observability"
)

const tracerName = "github.com/shanbay/gobay/extensions/busext"

var propagator = propagation.TraceContext{}

// HandlerWithContext 是 Handler 的可选扩展接口：实现它的 handler 在消费时拿到
// 携带上游 trace 上下文（及本次 Consumer span）的 ctx，把 ctx 传给 ent/redis/RPC
// 等下游调用即可让整条链路挂到同一个 trace 上。
// 未实现该接口的 handler 行为不变（走 Run()）；接口分发不依赖 OTEL_ENABLE，
// 未开启时 ctx 为 context.Background()。
type HandlerWithContext interface {
	Handler
	RunWithContext(ctx context.Context) error
}

// runHandler 优先走 HandlerWithContext，未实现则退回 Run()。
func runHandler(ctx context.Context, handler Handler) error {
	if h, ok := handler.(HandlerWithContext); ok {
		return h.RunWithContext(ctx)
	}
	return handler.Run()
}

// PushWithContext 与 Push 相同，但在 OTEL_ENABLE 开启时额外创建 SpanKind=Producer
// 的 span，并把 traceparent 注入消息 Headers——消费方无论 Go（本扩展）还是
// Python（opentelemetry-instrumentation-celery）都能续到同一条 trace。
func (b *BusExt) PushWithContext(ctx context.Context, exchange, routingKey string, data amqp.Publishing) error {
	if !observability.GetOtelEnable() {
		return b.Push(exchange, routingKey, data)
	}
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(
		ctx, "send/"+routingKey,
		oteltrace.WithSpanKind(oteltrace.SpanKindProducer),
		oteltrace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", routingKey),
			attribute.String("messaging.operation", "publish"),
		))
	defer span.End()
	if data.Headers == nil {
		data.Headers = amqp.Table{}
	}
	propagator.Inject(ctx, observability.MapCarrier(data.Headers))
	err := b.Push(exchange, routingKey, data)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// otelDispatchStart 在消费侧从 delivery.Headers 提取上游 traceparent（没有则自建
// root trace），创建 SpanKind=Consumer 的 span，返回携带该 span 的 ctx（供
// HandlerWithContext 透传给下游）和收尾回调。回调按 dispatch 的结果状态结束
// span：非 "success" 一律标记 Error，描述为状态串——失败的 bus 事件因此能被
// collector 的 tail_sampling 保留。OTEL_ENABLE 未开启时 ctx 为 Background、回调
// 为 no-op。
func otelDispatchStart(delivery amqp.Delivery) (context.Context, func(status string)) {
	if !observability.GetOtelEnable() {
		return context.Background(), func(string) {}
	}
	ctx := propagator.Extract(context.Background(), observability.MapCarrier(delivery.Headers))
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(
		ctx, "run/"+delivery.RoutingKey,
		oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
		oteltrace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", delivery.RoutingKey),
			attribute.String("messaging.operation", "process"),
		))
	return ctx, func(status string) {
		span.SetAttributes(attribute.String("gobay.bus.status", status))
		if status != "success" {
			span.SetStatus(codes.Error, status)
		}
		span.End()
	}
}
