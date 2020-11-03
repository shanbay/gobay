package redisext_test

import (
	"context"
	"fmt"
	"time"

	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/redisext"
)

func ExampleRedisExt_CheckHealth() {
	redis := &redisext.RedisExt{NS: "redis_"}
	exts := map[gobay.Key]gobay.Extension{
		"redis": redis,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	err := redis.CheckHealth(context.Background())
	fmt.Println(err)
	// Output:
	// <nil>
}

func ExampleRedisExt_Set() {
	redis := &redisext.RedisExt{NS: "redis_"}
	exts := map[gobay.Key]gobay.Extension{
		"redis": redis,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	var key = "redisKey"
	err := redis.Client(context.Background()).Set(key, "hello", 10*time.Second).Err()
	fmt.Println(err)
	res, err := redis.Client(context.Background()).Get("redisKey").Result()
	fmt.Println(res, err)
	// Output:
	// <nil>
	// hello <nil>
}

func ExampleRedisExt_AddPrefix() {
	redis := &redisext.RedisExt{NS: "redis_"}
	exts := map[gobay.Key]gobay.Extension{
		"redis": redis,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	prefixKey := redis.AddPrefix("testRawKey")
	fmt.Println(prefixKey)
	// Output: github-redis.testRawKey
}

func ExampleRedisExt_AddPrefixNoPrefix() {
	redis := &redisext.RedisExt{NS: "redisnoprefix"}
	exts := map[gobay.Key]gobay.Extension{
		"redis": redis,
	}

	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	prefixKey := redis.AddPrefix("testNoPrefixKey")
	fmt.Println(prefixKey)
	// Output: testNoPrefixKey
}
