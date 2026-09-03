package redisext_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/redisext"
)

// 不配任何连接池参数时，应当拿到 gobay 的默认值而不是 go-redis 的
func TestInit_Defaults(t *testing.T) {
	// go-redis v6 的 PoolSize 默认值是 10*runtime.NumCPU()，在 10 核机器上恰好等于
	// gobay 要设的 100，此时下面的 PoolSize 断言无法区分这个值是谁设的。NumCPU 不像
	// GOMAXPROCS 那样能在进程内改写，所以只能把这个退化显式报出来——CI runner 的核数
	// 不是 10，断言在那里是有效的。
	if 10*runtime.NumCPU() == 100 {
		t.Logf("注意：本机 NumCPU=%d，go-redis 默认 PoolSize 恰好也是 100，"+
			"本用例的 PoolSize 断言在此机器上不具区分度", runtime.NumCPU())
	}
	ext := &redisext.RedisExt{NS: "redis_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	o := ext.Client(context.Background()).Options()
	if o.PoolSize != 100 {
		t.Errorf("PoolSize = %d, want 100", o.PoolSize)
	}
	if o.ReadTimeout != 200*time.Millisecond {
		t.Errorf("ReadTimeout = %v, want 200ms", o.ReadTimeout)
	}
	if o.PoolTimeout != 100*time.Millisecond {
		t.Errorf("PoolTimeout = %v, want 100ms", o.PoolTimeout)
	}
	if o.IdleTimeout != 2*time.Minute {
		t.Errorf("IdleTimeout = %v, want 2m", o.IdleTimeout)
	}
}

// 显式配置优先于默认值
func TestInit_ExplicitWins(t *testing.T) {
	ext := &redisext.RedisExt{NS: "redisexplicit_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	o := ext.Client(context.Background()).Options()
	if o.PoolSize != 33 {
		t.Errorf("PoolSize = %d, want 33", o.PoolSize)
	}
	if o.ReadTimeout != 120*time.Millisecond {
		t.Errorf("ReadTimeout = %v, want 120ms", o.ReadTimeout)
	}
	if o.PoolTimeout != 60*time.Millisecond {
		t.Errorf("PoolTimeout = %v, want 60ms", o.PoolTimeout)
	}
	if o.IdleTimeout != 30*time.Second {
		t.Errorf("IdleTimeout = %v, want 30s", o.IdleTimeout)
	}
}

// 逃生门：显式配 0 让 go-redis 回填自己的默认值，不必回滚版本
func TestInit_PoolSizeEscapeHatch(t *testing.T) {
	ext := &redisext.RedisExt{NS: "redisescape_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	want := 10 * runtime.NumCPU()
	if o := ext.Client(context.Background()).Options(); o.PoolSize != want {
		t.Errorf("PoolSize = %d, want %d (go-redis 默认值)", o.PoolSize, want)
	}
}

// 带下划线的写法也要生效（历史上会被 mapstructure 静默忽略）
func TestInit_SnakeCaseAccepted(t *testing.T) {
	ext := &redisext.RedisExt{NS: "redisunderscore_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	o := ext.Client(context.Background()).Options()
	if o.PoolSize != 50 {
		t.Errorf("PoolSize = %d, want 50 — pool_size 应当生效", o.PoolSize)
	}
	if o.ReadTimeout != 150*time.Millisecond {
		t.Errorf("ReadTimeout = %v, want 150ms — read_timeout 应当生效", o.ReadTimeout)
	}
}

// 两种写法并存时以不带下划线的为准，结果必须确定
func TestInit_FlatKeyWinsOverSnakeCase(t *testing.T) {
	ext := &redisext.RedisExt{NS: "redisboth_"}
	exts := map[gobay.Key]gobay.Extension{"redis": ext}
	if _, err := gobay.CreateApp("../../testdata/", "redispool", exts); err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	defer ext.Close()

	if got := ext.Client(context.Background()).Options().PoolSize; got != 50 {
		t.Errorf("PoolSize = %d, want 50 — 并存时应以 poolsize 为准", got)
	}
}
