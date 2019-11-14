package cache

import (
	"github.com/go-redis/redis"
	"time"
)

type CacheBackend interface {
	SetClient(client interface{})
	Get(key string) (interface{}, error)
	Set(key string, value interface{}, ttl time.Duration) error
	SetMany(keys []string, values []interface{}, ttl time.Duration) error
	GetMany(keys []string) []interface{}
	Delete(key string) int64
	DeleteMany(keys []string) int64
	Expire(key string, ttl time.Duration) bool
	TTL(key string) int64
	Exists(key string) bool
	Clear() string
	Close() error
}

type redisBackend struct {
	client *redis.Client
}

type memBackend struct {
	client map[string]interface{}
	ttl    map[string]time.Duration
}

func (b *redisBackend) SetClient(client interface{}) {
	b.client = client.(*redis.Client)
}
func (b *redisBackend) Get(key string) (interface{}, error) {
	val, err := b.client.Get(key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return val, err
	}
	return val, nil
}

func (b *redisBackend) Set(key string, value interface{}, ttl time.Duration) error {
	return b.client.Set(key, value, ttl).Err()
}
func (b *redisBackend) SetMany(keys []string, values []interface{}, ttl time.Duration) error {
	pairs := make([]interface{}, 2*len(keys))
	for i, _ := range values {
		pairs = append(pairs, keys[i], values[i])
	}
	b.client.MSet(pairs...)
	for i, _ := range keys {
		b.client.Expire(keys[i], ttl)
	}
	return nil
}
func (b *redisBackend) GetMany(keys []string) []interface{} {
	return b.client.MGet(keys...).Val()
}
func (b *redisBackend) Delete(key string) int64 {
	keys := make([]string, 1)
	keys[0] = key
	return b.DeleteMany(keys)
}
func (b *redisBackend) DeleteMany(keys []string) int64 {
	return b.client.Del(keys...).Val()
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
	exists := (res.Val() == 1)
	return exists
}
func (b *redisBackend) Clear() string {
	return b.client.FlushDB().Val()
}
func (b *redisBackend) Close() error {
	return b.client.Close()
}

func (m *memBackend) SetClient(client interface{}) {
	m.client = client.(map[string]interface{})
	m.ttl = make(map[string]time.Duration)
}
func (m *memBackend) Get(key string) (interface{}, error) {
	res, exists := m.client[key]
	if exists {
		return res, nil
	} else {
		return nil, nil
	}
}

func (m *memBackend) Set(key string, value interface{}, ttl time.Duration) error {
	m.client[key] = value
	m.ttl[key] = ttl
	return nil
}
func (m *memBackend) SetMany(keys []string, values []interface{}, ttl time.Duration) error {
	for i, _ := range keys {
		m.client[keys[i]] = values[i]
		m.ttl[keys[i]] = ttl
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
func (m *memBackend) Delete(key string) int64 {
	exists := m.Exists(key)
	delete(m.client, key)
	delete(m.ttl, key)
	if exists {
		return 1
	} else {
		return 0
	}
}
func (m *memBackend) DeleteMany(keys []string) int64 {
	var res int64
	for _, key := range keys {
		if m.Delete(key) == 1 && res == 0 {
			res = 1
		}
	}
	return res
}
func (m *memBackend) Expire(key string, ttl time.Duration) bool {
	m.ttl[key] = ttl
	return true
}
func (m *memBackend) TTL(key string) int64 {
	ttl := m.ttl[key]
	return int64(ttl.Seconds())
}
func (m *memBackend) Exists(key string) bool {
	val, _ := m.Get(key)
	if val == nil {
		return false
	} else {
		return true
	}
}
func (m *memBackend) Clear() string {
	m.client = *new(map[string]interface{})
	m.ttl = *new(map[string]time.Duration)
	return ""
}
func (m *memBackend) Close() error {
	return nil
}
