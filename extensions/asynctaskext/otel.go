package asynctaskext

import (
	"context"

	"github.com/RichardKnop/machinery/v1/tasks"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/shanbay/gobay/observability"
)

const tracerName = "github.com/shanbay/gobay/extensions/asynctaskext"

var propagator = propagation.TraceContext{}

// otelSendStart 在发送侧创建 SpanKind=Producer 的 span，并把 traceparent 注入
// 任务消息头（machinery 会把 Headers 随消息序列化到 broker），供消费侧接续调用链。
// 返回带 span 的 ctx 和结束回调；OTEL_ENABLE 未开启时原样返回、回调为 no-op。
func otelSendStart(ctx context.Context, sign *tasks.Signature) (context.Context, func(err error)) {
	if !observability.GetOtelEnable() {
		return ctx, func(error) {}
	}
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(
		ctx, "send/"+sign.Name, oteltrace.WithSpanKind(oteltrace.SpanKindProducer))
	if sign.Headers == nil {
		sign.Headers = tasks.Headers{}
	}
	propagator.Inject(ctx, observability.MapCarrier(sign.Headers))
	return ctx, func(err error) {
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

// otelTaskStart / otelTaskEnd 挂在 machinery worker 的 Pre/PostTaskHandler 上：
// 从任务消息头提取上游 traceparent（没有则自建 root trace），为每次任务执行记录
// 一个 SpanKind=Consumer 的 span，按 signature.UUID 关联起止。
// 与 recordTaskDuration 同理，machinery 的全局 hook 拿不到该次调用的 error，
// span 状态不标 Error（保持 Unset）。
func (t *AsyncTaskExt) otelTaskStart(sig *tasks.Signature) {
	if !observability.GetOtelEnable() {
		return
	}
	ctx := propagator.Extract(context.Background(), observability.MapCarrier(sig.Headers))
	_, span := otel.GetTracerProvider().Tracer(tracerName).Start(
		ctx, "run/"+sig.Name, oteltrace.WithSpanKind(oteltrace.SpanKindConsumer))
	t.taskSpansM.Lock()
	defer t.taskSpansM.Unlock()
	if t.taskSpans == nil {
		t.taskSpans = make(map[string]oteltrace.Span)
	}
	t.taskSpans[sig.UUID] = span
}

func (t *AsyncTaskExt) otelTaskEnd(sig *tasks.Signature) {
	t.taskSpansM.Lock()
	span, ok := t.taskSpans[sig.UUID]
	if ok {
		delete(t.taskSpans, sig.UUID)
	}
	t.taskSpansM.Unlock()
	if ok {
		span.End()
	}
}
