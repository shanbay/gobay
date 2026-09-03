package redis

import (
	"context"
	"errors"
	"log"
	"runtime"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"

	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/cachext"
	"github.com/shanbay/gobay/observability"
)

// go-redis 默认的 PoolSize 是 10*runtime.GOMAXPROCS(0)。当 GOMAXPROCS 仍等于宿主机
// 核数时，多核节点上的容器会拿到一个远超实际需要的池上限。这个上限平时看不出来，
// 一旦 redis 变慢就会变成放大器：请求堆积 -> 建更多连接 -> redis 更慢。
//
// 取值来自实测：单池瞬时连接数峰值不足 100，取整到 100 可覆盖而不触顶。注意不能用
// 平均值推导——「实例总连接 ÷ pod 数 ÷ 池数」会同时抹平 pod 间负载不均和瞬时并发，
// 算出来比真实峰值小一个数量级，据此设的上限会误伤正常流量。
const defaultPoolSize = 100

// v9 的 pool.Conn.deadline 取 min(ctx.Deadline(), now+ReadTimeout)，waitTurn 也接受
// ctx，所以上游预算能自动传导。但两者是互补关系而非替代：ReadTimeout 是静态上界，
// 兜住没有 deadline 的调用（离线任务、cronjob），不设的话它们会落在 3 秒的默认值上；
// ctx 是动态剩余预算，上游已消耗大部分时间时能比静态上界更早放弃。
//
// 取值与 v6 backend 保持一致：正常 KV 操作在亚毫秒量级，200ms 留三个数量级余量；
// PoolTimeout 取其一半，最坏单次操作 300ms。
const (
	defaultReadTimeout = 200 * time.Millisecond
	defaultPoolTimeout = 100 * time.Millisecond
)

// go-redis v9 的 ConnMaxIdleTime 默认是 30 分钟，一次瞬时并发建出来的连接会被留到
// 半小时后才回收，池子只涨不落。收敛到 2 分钟，让峰值过去后连接数能跟着回落。
const defaultConnMaxIdleTime = 2 * time.Minute

// cpuLimitAware 报告 GOMAXPROCS 是否已经反映了容器的 CPU 限额。
//
// Go 1.25 起 GOMAXPROCS 默认取 min(可用 CPU 数, cgroup 的 quota/period)，但该行为由
// containermaxprocs GODEBUG 控制，而它的默认值取决于**主模块**（业务项目）go.mod 里的
// go 指令——gobay 作为依赖库读不到那个值，所以不能靠 runtime.Version() 判断：工具链是
// 1.25 而业务 go.mod 仍写 1.24 时，GOMAXPROCS 依然是宿主机核数。
//
// 因此直接看结果：GOMAXPROCS 已经小于 NumCPU，说明确实有机制把它调下来了（Go 1.25 的
// container-aware GOMAXPROCS，或 automaxprocs）。这时 go-redis 自己算出的
// 10*GOMAXPROCS 会随容器规格缩放，比一个固定值更合理，就不再覆盖它。
//
// 反过来两者相等时无法区分「没有 CPU 限额」和「限额恰好等于节点核数」，一律按未生效
// 处理，用固定默认值兜底——这个方向的误判是安全的。
func cpuLimitAware() bool {
	return runtime.GOMAXPROCS(0) < runtime.NumCPU()
}

func init() {
	if err := cachext.RegisterBackend("redis", func() cachext.CacheBackend { return &redisBackend{} }); err != nil {
		panic("RedisBackend init error")
	}
}

type redisBackend struct {
	client *redis.Client
}

func (b *redisBackend) Init(config *viper.Viper) error {
	// 先补齐下划线写法（<NS>pool_size），否则它会被 mapstructure 静默忽略。
	// 必须在 IsSet / SetDefault 之前，否则感知不到用户配的是哪种写法。
	gobay.NormalizeUnderscoreKeys(config)
	// IsSet 必须在 SetDefault 之前取，否则恒为 true，日志就分不清是用户配的还是这里兜的
	poolSizeConfigured := config.IsSet("poolsize")
	trustGoRedis := cpuLimitAware()
	if !trustGoRedis {
		config.SetDefault("poolsize", defaultPoolSize)
	}
	// 超时与 CPU 核数无关，无论 GOMAXPROCS 是否已感知容器限额都要设
	config.SetDefault("readtimeout", defaultReadTimeout)
	config.SetDefault("pooltimeout", defaultPoolTimeout)
	config.SetDefault("connmaxidletime", defaultConnMaxIdleTime)

	opt := redis.Options{}
	if err := config.Unmarshal(&opt); err != nil {
		return err
	}
	// redis.Options 只有 Addr 没有 Host，而 cachext 历史配置键是 <ns>host。
	// 不兜底的话 Addr 为空，go-redis 会静默 fallback 到 localhost:6379。
	if opt.Addr == "" {
		opt.Addr = config.GetString("host")
	}
	if opt.Addr == "" {
		return errors.New("missing config key `addr` (or legacy `host`)")
	}
	if !poolSizeConfigured {
		if trustGoRedis {
			log.Printf("[gobay/cachext] redis %s: poolsize 未配置；GOMAXPROCS=%d 已反映容器 CPU 限额"+
				"（NumCPU=%d），沿用 go-redis 默认值 10*GOMAXPROCS=%d",
				opt.Addr, runtime.GOMAXPROCS(0), runtime.NumCPU(), 10*runtime.GOMAXPROCS(0))
		} else {
			log.Printf("[gobay/cachext] redis %s: poolsize 未配置，使用 gobay 默认值 %d"+
				"（GOMAXPROCS=%d 未反映容器 CPU 限额，go-redis 默认会取 10*GOMAXPROCS=%d）；"+
				"如需恢复 go-redis 默认值，显式配置 <ns>poolsize: 0",
				opt.Addr, defaultPoolSize, runtime.GOMAXPROCS(0), 10*runtime.GOMAXPROCS(0))
		}
	}

	redisClient := redis.NewClient(&opt)
	b.client = redisClient
	if observability.GetOtelEnable() {
		tp := otel.GetTracerProvider()
		if err := redisotel.InstrumentTracing(redisClient, redisotel.WithTracerProvider(tp)); err != nil {
			return err
		}
	}
	_, err := redisClient.Ping(context.Background()).Result()
	return err
}

func (b *redisBackend) CheckHealth(ctx context.Context) error {
	_, err := b.client.Ping(ctx).Result()
	if err != nil {
		return err
	}
	return nil
}

func (b *redisBackend) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := b.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	return ([]byte)(val), nil
}

func (b *redisBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return b.client.Set(ctx, key, value, ttl).Err()
}

func (b *redisBackend) SetMany(ctx context.Context, keyValues map[string][]byte, ttl time.Duration) error {
	pairs := make([]interface{}, 2*len(keyValues))
	for key, value := range keyValues {
		pairs = append(pairs, key, value)
	}
	b.client.MSet(ctx, pairs...)
	for key := range keyValues {
		b.client.Expire(ctx, key, ttl)
	}
	return nil
}

func (b *redisBackend) GetMany(ctx context.Context, keys []string) [][]byte {
	res := make([][]byte, len(keys))
	for i, value := range b.client.MGet(ctx, keys...).Val() {
		if value != nil {
			res[i] = ([]byte)(value.(string))
		}
	}
	return res
}

func (b *redisBackend) Delete(ctx context.Context, key string) bool {
	keys := make([]string, 1)
	keys[0] = key
	return b.DeleteMany(ctx, keys)
}

func (b *redisBackend) DeleteMany(ctx context.Context, keys []string) bool {
	return b.client.Del(ctx, keys...).Val() == 1
}

func (b *redisBackend) Expire(ctx context.Context, key string, ttl time.Duration) bool {
	return b.client.Expire(ctx, key, ttl).Val()
}

func (b *redisBackend) TTL(ctx context.Context, key string) time.Duration {
	return b.client.TTL(ctx, key).Val()
}

func (b *redisBackend) Exists(ctx context.Context, key string) bool {
	keys := make([]string, 1)
	keys[0] = key
	res := b.client.Exists(ctx, keys...)
	return res.Val() == 1
}

func (b *redisBackend) Close() error {
	return b.client.Close()
}
