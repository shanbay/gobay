package redis

import (
	"runtime"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/shanbay/gobay"
)

const testdataDir = "../../../../../testdata/"

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

	want := 10 * runtime.GOMAXPROCS(0)
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

// GOMAXPROCS 已经小于 NumCPU 时（Go 1.25 的 container-aware GOMAXPROCS 生效，
// 或引入了 automaxprocs），go-redis 自算的 10*GOMAXPROCS 会随容器规格缩放，
// 比固定值更合理，此时不该再用 gobay 的默认值覆盖它。
func TestInit_SkipsDefaultWhenCPULimitAware(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("需要至少 2 个 CPU 才能制造 GOMAXPROCS < NumCPU")
	}
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host": "127.0.0.1:6379",
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	// 期望拿到 go-redis 自己算的 10*GOMAXPROCS = 10，而不是 gobay 的 20
	if got := b.client.Options().PoolSize; got != 10 {
		t.Errorf("PoolSize = %d, want 10 (10*GOMAXPROCS，不该被 gobay 默认值 %d 覆盖)", got, defaultPoolSize)
	}
}

// 显式配置在任何情况下都优先，包括 GOMAXPROCS 已感知 CPU 限额时
func TestInit_ExplicitPoolSizeWinsWhenCPULimitAware(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("需要至少 2 个 CPU")
	}
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host":     "127.0.0.1:6379",
		"poolsize": 33,
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	if got := b.client.Options().PoolSize; got != 33 {
		t.Errorf("PoolSize = %d, want 33", got)
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
	if defaultConnMaxIdleTime != 2*time.Minute {
		t.Errorf("defaultConnMaxIdleTime = %v, want 2m", defaultConnMaxIdleTime)
	}
}

// v9 的 deadline 取 min(ctx.Deadline(), now+ReadTimeout)：ReadTimeout 是静态上界，
// 兜住没有 deadline 的离线调用；ctx 是动态剩余预算。两者互补，所以 v9 同样要设。
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
	if o.ConnMaxIdleTime != defaultConnMaxIdleTime {
		t.Errorf("ConnMaxIdleTime = %v, want %v", o.ConnMaxIdleTime, defaultConnMaxIdleTime)
	}
}

func TestInit_ExplicitTimeoutsWin(t *testing.T) {
	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host":            "127.0.0.1:6379",
		"readtimeout":     "120ms",
		"pooltimeout":     "60ms",
		"connmaxidletime": "30s",
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
	if o.ConnMaxIdleTime != 30*time.Second {
		t.Errorf("ConnMaxIdleTime = %v, want 30s", o.ConnMaxIdleTime)
	}
}

// 超时默认值与 CPU 核数无关，即使 GOMAXPROCS 已感知容器限额（PoolSize 交还给
// go-redis 自己算）也必须照常生效
func TestInit_TimeoutsApplyWhenCPULimitAware(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("需要至少 2 个 CPU")
	}
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	b := &redisBackend{}
	if err := b.Init(cacheConfig(map[string]interface{}{
		"host": "127.0.0.1:6379",
	})); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer b.Close()

	o := b.client.Options()
	if o.PoolSize != 10 {
		t.Errorf("PoolSize = %d, want 10 (10*GOMAXPROCS，不该被 gobay 默认值覆盖)", o.PoolSize)
	}
	if o.ReadTimeout != defaultReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v（超时与 CPU 核数无关，必须照常生效）",
			o.ReadTimeout, defaultReadTimeout)
	}
}
