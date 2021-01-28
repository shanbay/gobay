package ratelimitext_test

import (
	"context"
	"fmt"
	"time"

	"github.com/shanbay/gobay/extensions/ratelimitext"
	_ "github.com/shanbay/gobay/extensions/ratelimitext/backend/redis"

	"github.com/shanbay/gobay"
)

func ExampleRatelimitExt_Allow_1_1() {
	ratelimit := &ratelimitext.RatelimitExt{NS: "ratelimit_"}
	exts := map[gobay.Key]gobay.Extension{
		"ratelimit": ratelimit,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	pathKey := "cache_key/1/1"
	allowed, err := ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 1, 1)
	fmt.Println(allowed, err)
	allowed, err = ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 1, 1)
	fmt.Println(allowed, err)

	// same pathKey diff IP
	allowed, err = ratelimit.Allow(context.Background(), pathKey, "192.168.0.2", 1, 1)
	fmt.Println(allowed, err)

	// same diff pathKey, same IP
	allowed, err = ratelimit.Allow(context.Background(), pathKey+"/1", "192.168.0.1", 1, 1)
	fmt.Println(allowed, err)

	// Output:
	// true <nil>
	// false <nil>
	// true <nil>
	// true <nil>
}

func ExampleRatelimitExt_Allow_1_2() {
	ratelimit := &ratelimitext.RatelimitExt{NS: "ratelimit_"}
	exts := map[gobay.Key]gobay.Extension{
		"ratelimit": ratelimit,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	pathKey := "cache_key/1/2"
	for i := 0; i < 5; i++ {
		allowed, err := ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
		fmt.Println(allowed, err)
	}
	allowed, err := ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
	fmt.Println(allowed, err)
	time.Sleep(1000 * time.Millisecond)
	for i := 0; i < 5; i++ {
		allowed, err := ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
		fmt.Println(allowed, err)
	}
	allowed, err = ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
	fmt.Println(allowed, err)

	// Output:
	// true <nil>
	// true <nil>
	// true <nil>
	// true <nil>
	// true <nil>
	// false <nil>
	// true <nil>
	// true <nil>
	// true <nil>
	// true <nil>
	// true <nil>
	// false <nil>
}

func ExampleRatelimitExt_Allow_1_3() {
	ratelimit := &ratelimitext.RatelimitExt{NS: "ratelimit_"}
	exts := map[gobay.Key]gobay.Extension{
		"ratelimit": ratelimit,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	pathKey := "cache_key/1/3"
	for i := 0; i < 5; i++ {
		allowed, err := ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
		fmt.Println(allowed, err)
	}
	allowed, err := ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
	fmt.Println(allowed, err)
	time.Sleep(500 * time.Millisecond) // 0.5s recovers 2/5 requests
	for i := 0; i < 2; i++ {
		allowed, err := ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
		fmt.Println(allowed, err)
	}
	allowed, err = ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
	fmt.Println(allowed, err)
	time.Sleep(500 * time.Millisecond) // 0.5s recovers the other 3/5 requests
	for i := 0; i < 3; i++ {
		allowed, err = ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
		fmt.Println(allowed, err)
	}
	allowed, err = ratelimit.Allow(context.Background(), pathKey, "192.168.0.1", 5, 1)
	fmt.Println(allowed, err)

	// Output:
	// true <nil>
	// true <nil>
	// true <nil>
	// true <nil>
	// true <nil>
	// false <nil>
	// true <nil>
	// true <nil>
	// false <nil>
	// true <nil>
	// true <nil>
	// true <nil>
	// false <nil>
}

// Note: this test takes too long, (20 seconds, with 30s test timeout).
// func ExampleRatelimitExt_Allow_2() {
// 	ratelimit := &ratelimitext.RatelimitExt{NS: "ratelimit_"}
// 	exts := map[gobay.Key]gobay.Extension{
// 		"ratelimit": ratelimit,
// 	}
// 	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
// 		fmt.Println(err)
// 		return
// 	}

// 	key := "cache_key-2"
// 	allowed, err := ratelimit.Allow(context.Background(), key, "192.168.0.1", 3, 60)
// 	fmt.Println(allowed, err)
// 	allowed, err = ratelimit.Allow(context.Background(), key, "192.168.0.1", 3, 60)
// 	fmt.Println(allowed, err)
// 	allowed, err = ratelimit.Allow(context.Background(), key, "192.168.0.1", 3, 60)
// 	fmt.Println(allowed, err)
// 	allowed, err = ratelimit.Allow(context.Background(), key, "192.168.0.1", 3, 60)
// 	fmt.Println(allowed, err)
// 	time.Sleep(10 * time.Second)
// 	allowed, err = ratelimit.Allow(context.Background(), key, "192.168.0.1", 3, 60)
// 	fmt.Println(allowed, err)
// 	time.Sleep(10 * time.Second)
// 	allowed, err = ratelimit.Allow(context.Background(), key, "192.168.0.1", 3, 60)
// 	fmt.Println(allowed, err)
// 	allowed, err = ratelimit.Allow(context.Background(), key, "192.168.0.1", 3, 60)
// 	fmt.Println(allowed, err)

// 	// Output:
// 	// true <nil>
// 	// true <nil>
// 	// true <nil>
// 	// false <nil>
// 	// false <nil>
// 	// true <nil>
// 	// false <nil>
// }

func ExampleRatelimitExt_CheckHealth() {
	ratelimit := &ratelimitext.RatelimitExt{NS: "ratelimit_"}
	exts := map[gobay.Key]gobay.Extension{
		"ratelimit": ratelimit,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	err := ratelimit.CheckHealth(context.Background())
	fmt.Println(err)
	// Output:
	// <nil>
}
