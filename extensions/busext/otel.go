package busext

import (
	"context"

	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/shanbay/gobay/observability"
)

const tracerName = "github.com/shanbay/gobay/extensions/busext"

var propagator = propagation.TraceContext{}

// PushWithContext 与 Push 相同，但在 OTEL_ENABLE 开启时额外创建 SpanKind=Producer
// 的 span，并把 traceparent 注入消息 Headers，供消费侧接续调用链。
func (b *BusExt) PushWithContext(ctx context.Context, exchange, routingKey string, data amqp.Publishing) error {
	if !observability.GetOtelEnable() {
		return b.Push(exchange, routingKey, data)
	}
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(
		ctx, "send/"+routingKey, oteltrace.WithSpanKind(oteltrace.SpanKindProducer))
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
// root trace），创建 SpanKind=Consumer 的 span。返回的回调按 dispatch 的结果状态
// 结束 span：非 "success" 一律标记 Error，描述为状态串。
// OTEL_ENABLE 未开启时回调为 no-op。
func otelDispatchStart(delivery amqp.Delivery) func(status string) {
	if !observability.GetOtelEnable() {
		return func(string) {}
	}
	ctx := propagator.Extract(context.Background(), observability.MapCarrier(delivery.Headers))
	_, span := otel.GetTracerProvider().Tracer(tracerName).Start(
		ctx, "run/"+delivery.RoutingKey, oteltrace.WithSpanKind(oteltrace.SpanKindConsumer))
	return func(status string) {
		if status != "success" {
			span.SetStatus(codes.Error, status)
		}
		span.End()
	}
}
