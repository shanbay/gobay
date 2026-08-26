// Package redisotelv6 为 go-redis v6 客户端提供 OpenTelemetry 链路追踪。
//
// go-redis v9 有官方的 redisotel，v6 没有：v6 的命令方法不接收 context，
// 唯一能拿到 ctx 的地方是 (*redis.Client).WithContext 返回的每请求副本。本包在该
// 副本上用 WrapProcess / WrapProcessPipeline 挂钩子，钩子闭包捕获 ctx —— 这与
// go.elastic.co/apm/module/apmgoredis 的做法完全一致，因此两者可以叠加在同一个
// 副本上，APM 与 OTel 各自产出 span，不必二选一。
//
// 之所以只能作用于每请求副本：WrapProcess 是 (c.process = fn(c.process)) 的原地
// 修改。go-redis v6 的 Client 值嵌入 baseClient，clone() 走值拷贝，因此在副本上
// 挂钩子不会影响基础 client、也不会跨请求累积。
//
// span 名与属性对齐 redisotel/v9，便于 v6 / v9 两条路径的数据在同一套看板里查询。
package redisotelv6

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-redis/redis"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerName 是本包创建 span 时使用的 instrumentation name。
const TracerName = "github.com/shanbay/gobay/observability/redisotelv6"

// 与 redisotel/v9 依赖的 rediscmd.AppendCmd 保持一致的截断口径。
const (
	numArgLimit = 32
	argLenLimit = 64
)

// WrapClient 在 client 上挂 OTel 追踪钩子并返回它。
//
// 传入的必须是 (*redis.Client).WithContext(ctx) 返回的**每请求副本**（gobay 的
// redisext.Client / cachext 的 withContext 都是这样拿的），否则钩子会挂到共享的
// 基础 client 上并跨请求累积。
//
// 仅当 ctx 中已有「有效且被采样」的 span 上下文时才产出 span，与 entext 里
// otelsql 的 SpanFilter 口径一致：既避免在无 trace 上下文时产生大量孤儿 root
// span，也保证被采样的链路是完整的。这是与 redisotel/v9 的一处有意差异。
func WrapClient(ctx context.Context, client *redis.Client) *redis.Client {
	if client == nil {
		return nil
	}
	if !shouldTrace(ctx) {
		return client
	}
	tracer := otel.GetTracerProvider().Tracer(TracerName)

	client.WrapProcess(func(oldProcess func(redis.Cmder) error) func(redis.Cmder) error {
		return func(cmd redis.Cmder) error {
			_, span := tracer.Start(ctx, cmd.Name(),
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					semconv.DBSystemRedis,
					semconv.DBStatement(cmdString(cmd)),
				),
			)
			defer span.End()

			err := oldProcess(cmd)
			recordError(span, err)
			return err
		}
	})

	client.WrapProcessPipeline(func(oldProcess func([]redis.Cmder) error) func([]redis.Cmder) error {
		return func(cmds []redis.Cmder) error {
			_, span := tracer.Start(ctx, "redis.pipeline "+pipelineSummary(cmds),
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					semconv.DBSystemRedis,
					semconv.DBStatement(cmdsString(cmds)),
					attribute.Int("db.redis.num_cmd", len(cmds)),
				),
			)
			defer span.End()

			err := oldProcess(cmds)
			recordError(span, err)
			return err
		}
	})

	return client
}

// shouldTrace 与 entext 中 otelsql 的 SpanFilter 判据保持一致。
func shouldTrace(ctx context.Context) bool {
	sc := trace.SpanContextFromContext(ctx)
	return sc.IsValid() && sc.IsSampled()
}

// recordError 不把 redis.Nil（键不存在）当成错误，与 redisotel/v9 一致。
func recordError(span trace.Span, err error) {
	if err == nil || err == redis.Nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func pipelineSummary(cmds []redis.Cmder) string {
	switch len(cmds) {
	case 0:
		return "(empty)"
	case 1:
		return cmds[0].Name()
	default:
		return cmds[0].Name() + " ... " + cmds[len(cmds)-1].Name()
	}
}

func cmdsString(cmds []redis.Cmder) string {
	b := make([]byte, 0, 64)
	for i, cmd := range cmds {
		if i > numArgLimit {
			break
		}
		if i > 0 {
			b = append(b, '\n')
		}
		b = appendCmd(b, cmd)
	}
	return string(b)
}

func cmdString(cmd redis.Cmder) string {
	return string(appendCmd(make([]byte, 0, 64), cmd))
}

func appendCmd(b []byte, cmd redis.Cmder) []byte {
	for i, arg := range cmd.Args() {
		if i > numArgLimit {
			break
		}
		if i > 0 {
			b = append(b, ' ')
		}
		b = appendArg(b, arg)
	}
	if err := cmd.Err(); err != nil && err != redis.Nil {
		b = append(b, ": "...)
		b = append(b, err.Error()...)
	}
	return b
}

func appendArg(b []byte, v interface{}) []byte {
	switch v := v.(type) {
	case nil:
		return append(b, "<nil>"...)
	case string:
		return appendTruncated(b, v)
	case []byte:
		return appendTruncated(b, string(v))
	case int:
		return strconv.AppendInt(b, int64(v), 10)
	case int64:
		return strconv.AppendInt(b, v, 10)
	case uint64:
		return strconv.AppendUint(b, v, 10)
	case float64:
		return strconv.AppendFloat(b, v, 'f', -1, 64)
	case bool:
		return strconv.AppendBool(b, v)
	default:
		return appendTruncated(b, fmt.Sprint(v))
	}
}

func appendTruncated(b []byte, s string) []byte {
	if len(s) > argLenLimit {
		s = s[:argLenLimit]
	}
	return append(b, s...)
}
