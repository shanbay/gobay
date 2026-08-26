package redisext_test

import (
	"context"
	"testing"
	"time"

	"go.elastic.co/apm/apmtest"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/redisext"
)

// TestClientEmitsBothSpans 端到端验证：APM 与 OTel 双开时，一条经由
// RedisExt.Client(ctx) 执行的命令会在两套链路里各产出一个 span。
//
// 需要 testdata/config.yaml 里配置的 Redis 可达（与本包其他测试一致）。
func TestClientEmitsBothSpans(t *testing.T) {
	// 必须在 Init 之前设置：两个开关都是在 Init 里读环境变量的。
	t.Setenv("APM_ENABLE", "true")
	t.Setenv("OTEL_ENABLE", "true")

	ext := &redisext.RedisExt{NS: "redis_"}
	app, err := gobay.CreateApp("../../testdata/", "testing",
		map[gobay.Key]gobay.Extension{"redis": ext})
	if err != nil {
		t.Fatalf("CreateApp 失败（Redis 不可达？）: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })

	// 必须在 CreateApp 之后设置：OTEL_ENABLE=true 时 gobay 自身会在 app 初始化里
	// 调用 otel.SetTracerProvider，先设会被它覆盖掉。
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	const key = "gobay-redisext-tracing-test"

	_, apmSpans, _ := apmtest.WithTransaction(func(ctx context.Context) {
		ctx, parent := tp.Tracer("test").Start(ctx, "parent")
		defer parent.End()

		if err := ext.Client(ctx).Set(key, "hello", 10*time.Second).Err(); err != nil {
			t.Errorf("SET 失败: %v", err)
			return
		}
		if _, err := ext.Client(ctx).Get(key).Result(); err != nil {
			t.Errorf("GET 失败: %v", err)
		}
		ext.Client(ctx).Del(key)
	})

	var apmRedis int
	for _, s := range apmSpans {
		if s.Subtype == "redis" {
			apmRedis++
		}
	}
	if apmRedis == 0 {
		t.Errorf("APM 侧没有 redis span，共 %d 个 span", len(apmSpans))
	}

	otelRedis := map[string]int{}
	for _, s := range exp.GetSpans() {
		if s.Name != "parent" {
			otelRedis[s.Name]++
		}
	}
	t.Logf("APM redis span 数=%d；OTel span=%v", apmRedis, otelRedis)
	for _, want := range []string{"set", "get", "del"} {
		if otelRedis[want] == 0 {
			t.Errorf("OTel 侧缺少 %q span，实际 %v", want, otelRedis)
		}
	}
}
