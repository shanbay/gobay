package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"

	"github.com/shanbay/gobay/extensions/cachext"
	"github.com/shanbay/gobay/observability"
)

func init() {
	if err := cachext.RegisterBackend("redis", func() cachext.CacheBackend { return &redisBackend{} }); err != nil {
		panic("RedisBackend init error")
	}
}

type redisBackend struct {
	client *redis.Client
}

func (b *redisBackend) Init(config *viper.Viper) error {
	host := config.GetString("host")
	password := config.GetString("password")
	dbNum := config.GetInt("db")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       dbNum,
	})
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
