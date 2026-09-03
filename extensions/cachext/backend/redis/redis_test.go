package redis

import (
	"runtime"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/shanbay/gobay"
)

const testdataDir = "../../../../testdata/"

// 模拟 gobay.GetConfigByPrefix(app.Config(), "cache_", true) 之后 backend 收到的 viper：
// 前缀已被 trim，所以键名是 host / password / db 而不是 cache_host。
func cacheConfig(kv map[string]interface{}) *viper.Viper {
	c := viper.New()
	c.Set("backend", "redis")
	c.Set("prefix", "gobay-test")
	c.Set("monitor_enable", false)
	for k, v := range kv {
		c.Set(k, v)
	}
	return c
}

// 历史配置只有 host 没有 addr。redis.Options 里没有 Host 字段，
// 不做兜底的话 Addr 会是空的，go-redis 静默连 localhost:6379 —— 这里断言的是 Addr 而不是
// Init 的返回值，因为本机/CI 的 localhost:6379 上真有 redis，Ping 会成功，错地址靠 Ping 抓不出来。
func TestInit_LegacyHostKey(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host": "127.0.0.1:6379",
		"db":   3,
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	if got := b.client.Options().Addr; got != "127.0.0.1:6379" {
		t.Errorf("Addr = %q, want 127.0.0.1:6379", got)
	}
	if got := b.client.Options().DB; got != 3 {
		t.Errorf("DB = %d, want 3", got)
	}
}

// addr 与 host 并存时 addr 优先（host 是兼容用的旧键名）
func TestInit_AddrWins(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"addr": "127.0.0.1:6379",
		"host": "10.255.255.1:1",
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	if got := b.client.Options().Addr; got != "127.0.0.1:6379" {
		t.Errorf("Addr = %q, want 127.0.0.1:6379 (addr should win over host)", got)
	}
}

// 地址完全没配时直接报错，而不是让 go-redis 静默 fallback 到 localhost:6379
func TestInit_MissingAddr(t *testing.T) {
	b := &redisBackend{}
	err := b.Init(cacheConfig(nil))
	if err == nil {
		t.Fatal("Init should fail when neither addr nor host is configured")
	}
	if b.client != nil {
		t.Error("client should not be created when address is missing")
	}
}

func TestInit_DefaultPoolSize(t *testing.T) {
	// go-redis v6 的 PoolSize 默认值是 10*runtime.NumCPU()，在 10 核机器上恰好等于
	// gobay 要设的 100，此时下面的 PoolSize 断言无法区分这个值是谁设的。NumCPU 不像
	// GOMAXPROCS 那样能在进程内改写，所以只能把这个退化显式报出来——CI runner 的核数
	// 不是 10，断言在那里是有效的。
	if 10*runtime.NumCPU() == 100 {
		t.Logf("注意：本机 NumCPU=%d，go-redis 默认 PoolSize 恰好也是 100，"+
			"本用例的 PoolSize 断言在此机器上不具区分度", runtime.NumCPU())
	}
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host": "127.0.0.1:6379",
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	if got := b.client.Options().PoolSize; got != defaultPoolSize {
		t.Errorf("PoolSize = %d, want %d", got, defaultPoolSize)
	}
}

func TestInit_ExplicitPoolSize(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host":     "127.0.0.1:6379",
		"poolsize": 50,
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	if got := b.client.Options().PoolSize; got != 50 {
		t.Errorf("PoolSize = %d, want 50", got)
	}
}

// 逃生门：显式配 0 让 go-redis 回到它自己的默认值（10*NumCPU），
// 这样业务方不用回滚 gobay 版本就能退回旧行为
func TestInit_PoolSizeEscapeHatch(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host":     "127.0.0.1:6379",
		"poolsize": 0,
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	want := 10 * runtime.NumCPU()
	if got := b.client.Options().PoolSize; got != want {
		t.Errorf("PoolSize = %d, want %d (go-redis default)", got, want)
	}
}

func TestInit_PoolTimeout(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host":        "127.0.0.1:6379",
		"pooltimeout": "500ms",
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	if got := b.client.Options().PoolTimeout; got != 500*time.Millisecond {
		t.Errorf("PoolTimeout = %v, want 500ms", got)
	}
}

// mapstructure 只做大小写折叠、不做下划线归一化，所以 pool_size 匹配不上 PoolSize，
// 会被静默忽略。这条测试把这个行为钉住，免得有人以为支持 snake_case 而写出静默失效的配置。
// 带下划线的写法也要生效。mapstructure 只做大小写折叠、不做下划线归一化，
// 历史上 pool_size 会被静默忽略；现在由 gobay.NormalizeUnderscoreKeys 补齐。
func TestInit_SnakeCaseAccepted(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host":         "127.0.0.1:6379",
		"pool_size":    50,
		"read_timeout": "150ms",
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	o := b.client.Options()
	if o.PoolSize != 50 {
		t.Errorf("PoolSize = %d, want 50 — pool_size 应当生效", o.PoolSize)
	}
	if o.ReadTimeout != 150*time.Millisecond {
		t.Errorf("ReadTimeout = %v, want 150ms — read_timeout 应当生效", o.ReadTimeout)
	}
}

// 两种写法并存时以不带下划线的为准。交给 mapstructure 自行处理的话，两个键都会
// 匹配同一个字段，最终哪个生效取决于 map 迭代顺序——实测过，是不确定的。
func TestInit_FlatKeyWinsOverSnakeCase(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host":      "127.0.0.1:6379",
		"poolsize":  50,
		"pool_size": 77,
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	if got := b.client.Options().PoolSize; got != 50 {
		t.Errorf("PoolSize = %d, want 50 — 并存时应以 poolsize 为准，结果必须确定", got)
	}
}

// 端到端：走真实的 config.yaml + gobay.CreateApp + GetConfigByPrefix 的前缀 trim，
// 确认业务方写的 cache_poolsize（带前缀）确实能落到 redis.Options 上。
// 上面的单测直接 Set("poolsize") 跳过了 trim 这一环，抓不到前缀相关的问题。
func TestInit_EndToEndFromConfigFile(t *testing.T) {
	app, err := gobay.CreateApp(testdataDir, "cacheredis", map[gobay.Key]gobay.Extension{})
	if err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}

	config := gobay.GetConfigByPrefix(app.Config(), "cache_", true)
	b := &redisBackend{}
	if err := b.Init(config); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	opt := b.client.Options()
	if opt.Addr != "127.0.0.1:6379" {
		t.Errorf("Addr = %q, want 127.0.0.1:6379 (from cache_host)", opt.Addr)
	}
	if opt.DB != 3 {
		t.Errorf("DB = %d, want 3 (from cache_db)", opt.DB)
	}
	if opt.PoolSize != 7 {
		t.Errorf("PoolSize = %d, want 7 (from cache_poolsize)", opt.PoolSize)
	}
	if opt.PoolTimeout != 9*time.Second {
		t.Errorf("PoolTimeout = %v, want 9s (from cache_pooltimeout)", opt.PoolTimeout)
	}
}

// 取值是决策结果而非实现细节，显式钉住，改动时必须连带改测试
func TestDefaults_Values(t *testing.T) {
	if defaultPoolSize != 100 {
		t.Errorf("defaultPoolSize = %d, want 100", defaultPoolSize)
	}
	if defaultReadTimeout != 200*time.Millisecond {
		t.Errorf("defaultReadTimeout = %v, want 200ms", defaultReadTimeout)
	}
	if defaultPoolTimeout != 100*time.Millisecond {
		t.Errorf("defaultPoolTimeout = %v, want 100ms", defaultPoolTimeout)
	}
	if defaultIdleTimeout != 2*time.Minute {
		t.Errorf("defaultIdleTimeout = %v, want 2m", defaultIdleTimeout)
	}
}

// v6 的连接读写和排队都不接受 context，上游超时管不到 redis 调用，
// 这三个默认值是唯一的闸
func TestInit_DefaultTimeouts(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host": "127.0.0.1:6379",
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	o := b.client.Options()
	if o.ReadTimeout != defaultReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v", o.ReadTimeout, defaultReadTimeout)
	}
	if o.PoolTimeout != defaultPoolTimeout {
		t.Errorf("PoolTimeout = %v, want %v", o.PoolTimeout, defaultPoolTimeout)
	}
	if o.IdleTimeout != defaultIdleTimeout {
		t.Errorf("IdleTimeout = %v, want %v", o.IdleTimeout, defaultIdleTimeout)
	}
}

// 有更严预算的服务可以自己收紧，显式配置优先于默认值
func TestInit_ExplicitTimeoutsWin(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host":        "127.0.0.1:6379",
		"readtimeout": "120ms",
		"pooltimeout": "60ms",
		"idletimeout": "30s",
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	o := b.client.Options()
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
