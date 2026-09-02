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

	"github.com/shanbay/gobay/extensions/cachext"
	"github.com/shanbay/gobay/observability"
)

// go-redis 默认的 PoolSize 是 10*runtime.GOMAXPROCS(0)。未引入 automaxprocs、
// 且 go.mod 的 go 指令低于 1.25 时，GOMAXPROCS 仍是宿主机核数，不感知容器的 CPU limit，
// 所以在多核节点上跑的容器会拿到一个远超实际需要的池上限。这个上限平时看不出来，
// 一旦 redis 变慢就会变成放大器：请求堆积 -> 建更多连接 -> redis 更慢。
// 注：Go 1.25 起 GOMAXPROCS 会感知 cgroup CPU limit（需 go.mod 的 go 指令 >= 1.25），
// 届时 go-redis 自己算出的值会随容器规格缩放，可以考虑配 <NS>poolsize: 0 交还给它。
const defaultPoolSize = 20

func init() {
	if err := cachext.RegisterBackend("redis", func() cachext.CacheBackend { return &redisBackend{} }); err != nil {
		panic("RedisBackend init error")
	}
}

type redisBackend struct {
	client *redis.Client
}

func (b *redisBackend) Init(config *viper.Viper) error {
	// IsSet 必须在 SetDefault 之前取，否则恒为 true，日志就分不清是用户配的还是这里兜的
	poolSizeConfigured := config.IsSet("poolsize")
	config.SetDefault("poolsize", defaultPoolSize)

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
			"（go-redis 默认为 10*GOMAXPROCS=%d）；如需恢复 go-redis 默认值，显式配置 <ns>poolsize: 0",
			opt.Addr, defaultPoolSize, 10*runtime.GOMAXPROCS(0))
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
