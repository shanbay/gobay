package redisv9ext_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/shanbay/gobay"
	redisv9ext "github.com/shanbay/gobay/extensions/redisext/v9"
)

// 不配任何连接池参数时，应当拿到 gobay 的默认值而不是 go-redis 的
func TestInit_Defaults(t *testing.T) {
	// go-redis v9 的 PoolSize 默认值是 10*GOMAXPROCS(0)。在 10 核机器上它恰好等于
	// gobay 要设的 100，断言就分不出这个值是谁设的了（v6 那边因为用的是 NumCPU，
	// 无法在进程内规避，只能靠 CI 的核数不同）。这里把 GOMAXPROCS 压到 1，让
	// go-redis 的默认值变成 10，与 100 拉开距离，断言在任何机器上都有区分度。
	// redisext v9 无条件 SetDefault，不像 cachext v9 backend 那样有 cpuLimitAware
	// 分支，所以压低 GOMAXPROCS 不会改变它设默认值的行为。
	if runtime.NumCPU() < 2 {
		t.Skip("需要至少 2 个 CPU")
	}
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	ext := &redisv9ext.RedisExt{NS: "redis_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	o := ext.Client().Options()
	if o.PoolSize != 100 {
		t.Errorf("PoolSize = %d, want 100", o.PoolSize)
	}
	if o.ReadTimeout != 200*time.Millisecond {
		t.Errorf("ReadTimeout = %v, want 200ms", o.ReadTimeout)
	}
	if o.PoolTimeout != 100*time.Millisecond {
		t.Errorf("PoolTimeout = %v, want 100ms", o.PoolTimeout)
	}
	if o.ConnMaxIdleTime != 2*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 2m", o.ConnMaxIdleTime)
	}
}

// 显式配置优先于默认值
func TestInit_ExplicitWins(t *testing.T) {
	ext := &redisv9ext.RedisExt{NS: "redisexplicit_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	o := ext.Client().Options()
	if o.PoolSize != 33 {
		t.Errorf("PoolSize = %d, want 33", o.PoolSize)
	}
	if o.ReadTimeout != 120*time.Millisecond {
		t.Errorf("ReadTimeout = %v, want 120ms", o.ReadTimeout)
	}
	if o.PoolTimeout != 60*time.Millisecond {
		t.Errorf("PoolTimeout = %v, want 60ms", o.PoolTimeout)
	}
	if o.ConnMaxIdleTime != 30*time.Second {
		t.Errorf("ConnMaxIdleTime = %v, want 30s", o.ConnMaxIdleTime)
	}
}

// 带下划线的写法也要生效（历史上会被 mapstructure 静默忽略）
func TestInit_SnakeCaseAccepted(t *testing.T) {
	ext := &redisv9ext.RedisExt{NS: "redisunderscore_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	o := ext.Client().Options()
	if o.PoolSize != 50 {
		t.Errorf("PoolSize = %d, want 50 — pool_size 应当生效", o.PoolSize)
	}
	if o.ReadTimeout != 150*time.Millisecond {
		t.Errorf("ReadTimeout = %v, want 150ms — read_timeout 应当生效", o.ReadTimeout)
	}
}

// 两种写法并存时以不带下划线的为准，结果必须确定
func TestInit_FlatKeyWinsOverSnakeCase(t *testing.T) {
	ext := &redisv9ext.RedisExt{NS: "redisboth_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	if got := ext.Client().Options().PoolSize; got != 50 {
		t.Errorf("PoolSize = %d, want 50 — 并存时应以 poolsize 为准", got)
	}
}
