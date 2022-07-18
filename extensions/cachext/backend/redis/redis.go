package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis"
	"github.com/spf13/viper"
	"go.elastic.co/apm/module/apmgoredis"

	"github.com/shanbay/gobay/extensions/cachext"
)

func init() {
	if err := cachext.RegisteBackend("redis", func() cachext.CacheBackend { return &redisBackend{} }); err != nil {
		panic("RedisBackend init error")
	}
}

type redisBackend struct {
	client *redis.Client
}

func (b *redisBackend) withContext(ctx context.Context) *redis.Client {
	return apmgoredis.Wrap(b.client).WithContext(ctx).RedisClient()
}

func (b *redisBackend) Init(config *viper.Viper) error {
	opt := redis.Options{}
	if err := config.Unmarshal(&opt); err != nil {
		return err
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
