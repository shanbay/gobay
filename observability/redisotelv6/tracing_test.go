package redisotelv6

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"go.elastic.co/apm/apmtest"
	"go.elastic.co/apm/module/apmgoredis"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// 指向一个必定连不上的端口：命令一定失败，但 Cmder 与 span 照常产生，
// 因此这些测试不需要真实 Redis。
func deadClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		MaxRetries:  0,
		DialTimeout: 50 * time.Millisecond,
	})
}

func recorder(t *testing.T) (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	// WrapClient 从全局 TracerProvider 取 tracer，测试里必须把它设上，
	// 否则 span 会进 no-op provider。
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})
	return exp, tp
}

// sampledCtx 返回一个带「有效且被采样」span 的 ctx，并把该 span 结束掉，
// 避免它混进断言。
func sampledCtx(t *testing.T, tp *sdktrace.TracerProvider) context.Context {
	t.Helper()
	ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
	span.End()
	return ctx
}

func TestWrapClientEmitsSpan(t *testing.T) {
	exp, tp := recorder(t)
	ctx := sampledCtx(t, tp)

	base := deadClient()
	t.Cleanup(func() { _ = base.Close() })

	client := WrapClient(ctx, base.WithContext(ctx))
	_ = client.Get("some-key").Err() // 必然失败，但 span 应当产生

	var got trace.SpanKind
	var found bool
	var stmt string
	for _, s := range exp.GetSpans() {
		if s.Name == "parent" {
			continue
		}
		found = true
		got = s.SpanKind
		if s.Name != "get" {
			t.Errorf("span 名应为命令名 get，实际 %q", s.Name)
		}
		for _, a := range s.Attributes {
			switch string(a.Key) {
			case "db.system":
				if a.Value.AsString() != "redis" {
					t.Errorf("db.system 应为 redis，实际 %q", a.Value.AsString())
				}
			case "db.statement":
				stmt = a.Value.AsString()
			}
		}
		if s.Status.Code == 0 {
			t.Error("连接失败时 span 状态应为 Error")
		}
	}
	if !found {
		t.Fatal("没有产出 redis span")
	}
	if got != trace.SpanKindClient {
		t.Errorf("SpanKind 应为 client，实际 %v", got)
	}
	if !strings.HasPrefix(stmt, "get some-key") {
		t.Errorf("db.statement 应以命令和参数开头，实际 %q", stmt)
	}
}

// TestWrapClientSkipsWithoutSampledContext —— 无 trace 上下文时不产出 span，
// 避免大量孤儿 root span。这是与 redisotel/v9 的有意差异。
func TestWrapClientSkipsWithoutSampledContext(t *testing.T) {
	exp, _ := recorder(t)

	base := deadClient()
	t.Cleanup(func() { _ = base.Close() })

	ctx := context.Background()
	client := WrapClient(ctx, base.WithContext(ctx))
	_ = client.Get("some-key").Err()

	if n := len(exp.GetSpans()); n != 0 {
		t.Errorf("无采样上下文时不应产出 span，实际 %d 个", n)
	}
}

// TestWrapClientDoesNotLeakToBaseClient 是本包最关键的安全前提：
// 钩子必须只作用于 WithContext 返回的每请求副本。若它污染了基础 client，
// 钩子会跨请求累积（既是内存泄漏，也会让 span 绑到过期的 ctx 上）。
func TestWrapClientDoesNotLeakToBaseClient(t *testing.T) {
	exp, tp := recorder(t)
	ctx := sampledCtx(t, tp)

	base := deadClient()
	t.Cleanup(func() { _ = base.Close() })

	// 连续包装 5 个每请求副本
	for i := 0; i < 5; i++ {
		_ = WrapClient(ctx, base.WithContext(ctx))
	}
	exp.Reset()

	// 直接用基础 client 执行命令：不应产生任何 span
	_ = base.Get("some-key").Err()
	if n := len(exp.GetSpans()); n != 0 {
		t.Fatalf("基础 client 被污染了：产出了 %d 个 span，说明钩子会跨请求累积", n)
	}

	// 而每请求副本仍应正常产出，且恰好一个（没有重复挂钩）
	exp.Reset()
	client := WrapClient(ctx, base.WithContext(ctx))
	_ = client.Get("some-key").Err()
	if n := len(exp.GetSpans()); n != 1 {
		t.Fatalf("每请求副本应恰好产出 1 个 span，实际 %d 个", n)
	}
}

func TestAppendArgTruncates(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := string(appendArg(nil, long))
	if len(got) != argLenLimit {
		t.Errorf("长参数应截断到 %d 字节，实际 %d", argLenLimit, len(got))
	}
	if n := string(appendArg(nil, nil)); n != "<nil>" {
		t.Errorf("nil 参数应渲染为 <nil>，实际 %q", n)
	}
	if n := string(appendArg(nil, 42)); n != "42" {
		t.Errorf("int 参数应渲染为 42，实际 %q", n)
	}
}

// TestCoexistsWithAPM 验证本包与 apmgoredis 叠加在同一个每请求副本上时，
// 一条命令会在两套链路里各产出一个 span，而不是二选一。
//
// 这里的组合顺序与 redisext.Client / cachext 的 withContext 完全一致：
// 先 apmgoredis.Wrap(...).WithContext(ctx).RedisClient()，再 WrapClient(ctx, ...)。
// 命令打向死端口必然失败，但两个钩子都会照常建 span，因此无需真实 Redis。
func TestCoexistsWithAPM(t *testing.T) {
	exp, tp := recorder(t)

	base := deadClient()
	t.Cleanup(func() { _ = base.Close() })

	_, apmSpans, _ := apmtest.WithTransaction(func(ctx context.Context) {
		ctx, parent := tp.Tracer("test").Start(ctx, "parent")
		defer parent.End()

		client := apmgoredis.Wrap(base).WithContext(ctx).RedisClient()
		client = WrapClient(ctx, client)
		_ = client.Get("some-key").Err()
	})

	// —— APM 侧 ——
	var apmRedisSpan bool
	for _, s := range apmSpans {
		t.Logf("APM span: name=%q type=%q subtype=%q", s.Name, s.Type, s.Subtype)
		if s.Subtype == "redis" {
			apmRedisSpan = true
		}
	}
	if !apmRedisSpan {
		t.Errorf("APM 侧没有产出 redis span，共 %d 个", len(apmSpans))
	}

	// —— OTel 侧 ——
	var otelRedisSpan bool
	for _, s := range exp.GetSpans() {
		if s.Name == "parent" {
			continue
		}
		t.Logf("OTel span: name=%q kind=%v", s.Name, s.SpanKind)
		if s.Name == "get" {
			otelRedisSpan = true
		}
	}
	if !otelRedisSpan {
		t.Errorf("OTel 侧没有产出 redis span，共 %d 个", len(exp.GetSpans()))
	}
}
