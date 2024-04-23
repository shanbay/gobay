package redisv9ext_test

import (
	"context"
	"fmt"
	"time"

	"github.com/shanbay/gobay"
	redisv9ext "github.com/shanbay/gobay/extensions/redisext/v9"
	"go.opentelemetry.io/otel/trace"
)

func ExampleRedisExt_CheckHealth() {
	redis := &redisv9ext.RedisExt{NS: "redis_"}
	exts := map[gobay.Key]gobay.Extension{
		"redis": redis,
	}
	if _, err := gobay.CreateApp("../../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	err := redis.CheckHealth(context.Background())
	fmt.Println(err)
	// Output:
	// <nil>
}

func ExampleRedisExt_Set() {
	redis := &redisv9ext.RedisExt{NS: "redis_"}
	exts := map[gobay.Key]gobay.Extension{
		"redis": redis,
	}
	if _, err := gobay.CreateApp("../../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	var key = "redisKey"
	ctx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(context.Background()))
	err := redis.Client().Set(ctx, key, "hello", 10*time.Second).Err()
	fmt.Println(err)
	res, err := redis.Client().Get(ctx, "redisKey").Result()
	fmt.Println(res, err)
	// Output:
	// <nil>
	// hello <nil>
}

func ExampleRedisExt_AddPrefix() {
	redis := &redisv9ext.RedisExt{NS: "redis_"}
	exts := map[gobay.Key]gobay.Extension{
		"redis": redis,
	}
	if _, err := gobay.CreateApp("../../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	prefixKey := redis.AddPrefix("testRawKey")
	fmt.Println(prefixKey)
	// Output: github-redis.testRawKey
}

func ExampleRedisExt_AddPrefixNoPrefix() {
	redis := &redisv9ext.RedisExt{NS: "redisnoprefix"}
	exts := map[gobay.Key]gobay.Extension{
		"redis": redis,
	}

	if _, err := gobay.CreateApp("../../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	prefixKey := redis.AddPrefix("testNoPrefixKey")
	fmt.Println(prefixKey)
	// Output: testNoPrefixKey
}
