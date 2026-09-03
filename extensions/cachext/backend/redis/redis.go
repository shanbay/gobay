package redis

import (
	"context"
	"errors"
	"log"
	"runtime"
	"time"

	"github.com/go-redis/redis"
	"github.com/spf13/viper"
	"go.elastic.co/apm/module/apmgoredis"

	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/cachext"
	"github.com/shanbay/gobay/observability"
)

// go-redis 默认的 PoolSize 是 10*runtime.NumCPU()。NumCPU 读的是宿主机核数，
// 不感知容器的 CPU limit，所以在多核节点上跑的容器会拿到一个远超实际需要的池上限。
// 这个上限平时看不出来，一旦 redis 变慢就会变成放大器：请求堆积 -> 建更多连接 ->
// redis 更慢。收敛到一个固定值，把压力挡在服务侧。
// 注：runtime.NumCPU() 不受 GOMAXPROCS 影响，Go 1.25 的 container-aware GOMAXPROCS
// 也不会改变它，所以 v6 这边只能靠显式默认值。
//
// 取值来自实测：单池瞬时连接数峰值不足 100，取整到 100 可覆盖而不触顶。注意不能用
// 平均值推导——「实例总连接 ÷ pod 数 ÷ 池数」会同时抹平 pod 间负载不均和瞬时并发，
// 算出来比真实峰值小一个数量级，据此设的上限会误伤正常流量。
const defaultPoolSize = 100

// go-redis v6 的连接读写和排队都不接受 context：pool.Conn 结构体里没有 ctx 字段，
// WithReader 只认 ReadTimeout；waitTurn() 连参数都没有，只认 PoolTimeout。也就是说
// 上游 ctx 到期之后，这次调用占用的连接仍会等满 ReadTimeout 才归还，还在排队的再叠加
// 一个 PoolTimeout —— 一个已经被放弃的请求最坏能占住连接 7 秒（3s + 4s 的默认值）。
// redis 一变慢，这就是连接池只涨不落的直接原因。
//
// 按 Little's Law，连接数需求等于请求速率乘以单请求耗时，所以压缩耗时比限制连接数
// 更接近问题本源：PoolSize 只在池已打满后才生效，那时膨胀已经发生。
//
// 取值：正常 KV 操作在亚毫秒量级，200ms 仍留三个数量级的余量，同时不长于典型在线
// 链路的上游预算；PoolTimeout 取其一半，排队不应比执行更久，最坏单次操作 300ms。
// 有更严预算的服务在 <NS>readtimeout / <NS>pooltimeout 里自行收紧。
const (
	defaultReadTimeout = 200 * time.Millisecond
	defaultPoolTimeout = 100 * time.Millisecond
)

// 空闲连接回收。v6 的默认值是 5 分钟，本身不算长，收敛到 2 分钟是为了与 v9 backend
// 保持一致——v9 对应的 ConnMaxIdleTime 默认长达 30 分钟，一次瞬时并发建出来的连接会
// 被留到半小时后才回收，池子只涨不落。
const defaultIdleTimeout = 2 * time.Minute

func init() {
	if err := cachext.RegisterBackend("redis", func() cachext.CacheBackend { return &redisBackend{} }); err != nil {
		panic("RedisBackend init error")
	}
}

type redisBackend struct {
	client *redis.Client
}

func (b *redisBackend) withContext(ctx context.Context) *redis.Client {
	if observability.GetApmEnable() {
		return apmgoredis.Wrap(b.client).WithContext(ctx).RedisClient()
	}
	return b.client.WithContext(ctx)
}

func (b *redisBackend) Init(config *viper.Viper) error {
	// 先补齐下划线写法（<NS>pool_size），否则它会被 mapstructure 静默忽略。
	// 必须在 IsSet / SetDefault 之前，否则感知不到用户配的是哪种写法。
	gobay.NormalizeUnderscoreKeys(config)
	// IsSet 必须在 SetDefault 之前取，否则恒为 true，日志就分不清是用户配的还是这里兜的
	poolSizeConfigured := config.IsSet("poolsize")
	config.SetDefault("poolsize", defaultPoolSize)
	config.SetDefault("readtimeout", defaultReadTimeout)
	config.SetDefault("pooltimeout", defaultPoolTimeout)
	config.SetDefault("idletimeout", defaultIdleTimeout)

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
		log.Printf("[gobay/cachext] redis %s: poolsize 未配置，使用 gobay 默认值 %d"+
			"（go-redis 默认为 10*NumCPU=%d）；如需恢复 go-redis 默认值，显式配置 <ns>poolsize: 0",
			opt.Addr, defaultPoolSize, 10*runtime.NumCPU())
	}

	redisClient := redis.NewClient(&opt)
	b.client = redisClient
	_, err := redisClient.Ping().Result()
	return err
}

func (b *redisBackend) CheckHealth(ctx context.Context) error {
	client := b.withContext(ctx)
	_, err := client.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func (b *redisBackend) Get(ctx context.Context, key string) ([]byte, error) {
	client := b.withContext(ctx)
	val, err := client.Get(key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	return ([]byte)(val), nil
}

func (b *redisBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	client := b.withContext(ctx)
	return client.Set(key, value, ttl).Err()
}

func (b *redisBackend) SetMany(ctx context.Context, keyValues map[string][]byte, ttl time.Duration) error {
	pairs := make([]interface{}, 2*len(keyValues))
	for key, value := range keyValues {
		pairs = append(pairs, key, value)
	}
	client := b.withContext(ctx)
	client.MSet(pairs...)
	for key := range keyValues {
		client.Expire(key, ttl)
	}
	return nil
}

func (b *redisBackend) GetMany(ctx context.Context, keys []string) [][]byte {
	res := make([][]byte, len(keys))
	client := b.withContext(ctx)
	for i, value := range client.MGet(keys...).Val() {
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
	client := b.withContext(ctx)
	return client.Del(keys...).Val() == 1
}

func (b *redisBackend) Expire(ctx context.Context, key string, ttl time.Duration) bool {
	client := b.withContext(ctx)
	return client.Expire(key, ttl).Val()
}

func (b *redisBackend) TTL(ctx context.Context, key string) time.Duration {
	client := b.withContext(ctx)
	return client.TTL(key).Val()
}

func (b *redisBackend) Exists(ctx context.Context, key string) bool {
	keys := make([]string, 1)
	keys[0] = key
	client := b.withContext(ctx)
	res := client.Exists(keys...)
	return res.Val() == 1
}

func (b *redisBackend) Close() error {
	return b.client.Close()
}
