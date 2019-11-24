package redis

import (
	"github.com/go-redis/redis"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/cachext"
	"time"
)

func init() {
	if err := cachext.RegisteBackend("redis", &redisBackend{}); err != nil {
		panic("RedisBackend init error")
	}
}

type redisBackend struct {
	client *redis.Client
}

func (b *redisBackend) Init(app *gobay.Application) error {
	config := app.Config()
	host := config.GetString("cache_host")
	password := config.GetString("cache_password")
	dbNum := config.GetInt("cache_db")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       dbNum,
	})
	b.client = redisClient
	_, err := redisClient.Ping().Result()
	return err
}

func (b *redisBackend) Get(key string) ([]byte, error) {
	val, err := b.client.Get(key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	return ([]byte)(val), nil
}

func (b *redisBackend) Set(key string, value []byte, ttl time.Duration) error {
	return b.client.Set(key, value, ttl).Err()
}

func (b *redisBackend) SetMany(keyValues map[string][]byte, ttl time.Duration) error {
	pairs := make([]interface{}, 2*len(keyValues))
	for key, value := range keyValues {
		pairs = append(pairs, key, value)
	}
	b.client.MSet(pairs...)
	for key := range keyValues {
		b.client.Expire(key, ttl)
	}
	return nil
}

func (b *redisBackend) GetMany(keys []string) [][]byte {
	res := make([][]byte, len(keys))
	for i, value := range b.client.MGet(keys...).Val() {
		if value != nil {
			res[i] = ([]byte)(value.(string))
		}
	}
	return res
}

func (b *redisBackend) Delete(key string) bool {
	keys := make([]string, 1)
	keys[0] = key
	return b.DeleteMany(keys)
}

func (b *redisBackend) DeleteMany(keys []string) bool {
	return b.client.Del(keys...).Val() == 1
}

func (b *redisBackend) Expire(key string, ttl time.Duration) bool {
	return b.client.Expire(key, ttl).Val()
}

func (b *redisBackend) TTL(key string) time.Duration {
	return b.client.TTL(key).Val()
}

func (b *redisBackend) Exists(key string) bool {
	keys := make([]string, 1)
	keys[0] = key
	res := b.client.Exists(keys...)
	return res.Val() == 1
}

func (b *redisBackend) Close() error {
	return b.client.Close()
}
