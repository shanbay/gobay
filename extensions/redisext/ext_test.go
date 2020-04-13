package redisext_test

import (
	"fmt"
	"time"

	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/redisext"
)

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
	err := redis.Set(key, "hello", 10*time.Second).Err()
	fmt.Println(err)
	res, err := redis.Get("redisKey").Result()
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

	redis.NS = ""
	prefixKey = redis.AddPrefix("testNoPrefixKey")
	fmt.Println(prefixKey)
	// Output: testNoPrefixKey
}
