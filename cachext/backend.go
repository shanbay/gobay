package cachext

import (
	"github.com/go-redis/redis"
	"github.com/shanbay/gobay"
	"math"
	"time"
)

type CacheBackend interface {
	Init(app *gobay.Application) error
	Get(key string) (interface{}, error)
	Set(key string, value interface{}, ttl time.Duration) error
	SetMany(keyValues map[string]interface{}, ttl time.Duration) error
	GetMany(keys []string) []interface{}
	Delete(key string) bool
	DeleteMany(keys []string) bool
	Expire(key string, ttl time.Duration) bool
	TTL(key string) int64
	Exists(key string) bool
	Close() error
}

var _ CacheBackend = (*redisBackend)(nil)
var _ CacheBackend = (*memBackend)(nil)

type redisBackend struct {
	client *redis.Client
}

type memBackendNode struct {
	Value     interface{}
	ExpiredAt time.Time
}

type memBackend struct {
	client map[string]*memBackendNode
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

func (b *redisBackend) Get(key string) (interface{}, error) {
	val, err := b.client.Get(key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func (b *redisBackend) Set(key string, value interface{}, ttl time.Duration) error {
	return b.client.Set(key, value, ttl).Err()
}

func (b *redisBackend) SetMany(keyValues map[string]interface{}, ttl time.Duration) error {
	pairs := make([]interface{}, 2*len(keyValues))
	for key, value := range keyValues {
		pairs = append(pairs, key, value)
	}
	b.client.MSet(pairs...)
	for key, _ := range keyValues {
		b.client.Expire(key, ttl)
	}
	return nil
}

func (b *redisBackend) GetMany(keys []string) []interface{} {
	return b.client.MGet(keys...).Val()
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

func (b *redisBackend) TTL(key string) int64 {
	return b.client.TTL(key).Val().Milliseconds() / 1000
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

func (m *memBackend) Init(app *gobay.Application) error {
	m.client = make(map[string]*memBackendNode)
	return nil
}

func (m *memBackend) Get(key string) (interface{}, error) {
	res, exists := m.client[key]
	if exists == false {
		return nil, nil
	}
	if res.ExpiredAt.Before(time.Now()) {
		m.Delete(key)
		return nil, nil
	} else {
		return res.Value, nil
	}
}

func (m *memBackend) Set(key string, value interface{}, ttl time.Duration) error {
	node := &memBackendNode{Value: value, ExpiredAt: time.Now().Add(ttl)}
	m.client[key] = node
	return nil
}

func (m *memBackend) SetMany(keyValues map[string]interface{}, ttl time.Duration) error {
	for key, value := range keyValues {
		m.Set(key, value, ttl)
	}
	return nil
}

func (m *memBackend) GetMany(keys []string) []interface{} {
	res := make([]interface{}, len(keys))
	for i, key := range keys {
		res[i], _ = m.Get(key)
	}
	return res
}

func (m *memBackend) Delete(key string) bool {
	exists := m.Exists(key)
	delete(m.client, key)
	return exists
}

func (m *memBackend) DeleteMany(keys []string) bool {
	var res bool
	for _, key := range keys {
		if m.Delete(key) {
			res = true
		}
	}
	return res
}

func (m *memBackend) Expire(key string, ttl time.Duration) bool {
	val, _ := m.Get(key)
	if val == nil {
		return false
	}
	m.client[key].ExpiredAt = time.Now().Add(ttl)
	return true
}

func (m *memBackend) TTL(key string) int64 {
	_, _ = m.Get(key)
	val := m.client[key]
	if val == nil {
		return 0
	}
	return int64(math.Round(val.ExpiredAt.Sub(time.Now()).Seconds()))
}

func (m *memBackend) Exists(key string) bool {
	val, _ := m.Get(key)
	if val == nil {
		return false
	} else {
		return true
	}
}

func (m *memBackend) Close() error {
	return nil
}
