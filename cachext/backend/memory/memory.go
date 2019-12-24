package memory

import (
	"context"
	"github.com/shanbay/gobay/cachext"
	"github.com/spf13/viper"
	"time"
)

func init() {
	if err := cachext.RegisteBackend("memory", &memoryBackend{}); err != nil {
		panic("MemoryBackend Init error")
	}
}

type memoryBackendNode struct {
	Value     []byte
	ExpiredAt time.Time
}

type memoryBackend struct {
	client map[string]*memoryBackendNode
}

func (m *memoryBackend) Init(*viper.Viper) error {
	m.client = make(map[string]*memoryBackendNode)
	return nil
}

func (m *memoryBackend) WithContext(ctx context.Context) cachext.CacheBackend {
	return m
}

func (m *memoryBackend) Get(key string) ([]byte, error) {
	res, exists := m.client[key]
	if !exists {
		return nil, nil
	}
	if res.ExpiredAt.Before(time.Now()) {
		m.Delete(key)
		return nil, nil
	} else {
		return res.Value, nil
	}
}

func (m *memoryBackend) Set(key string, value []byte, ttl time.Duration) error {
	node := &memoryBackendNode{Value: value, ExpiredAt: time.Now().Add(ttl)}
	m.client[key] = node
	return nil
}

func (m *memoryBackend) SetMany(keyValues map[string][]byte, ttl time.Duration) error {
	for key, value := range keyValues {
		if err := m.Set(key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (m *memoryBackend) GetMany(keys []string) [][]byte {
	resBytes := make([][]byte, len(keys))
	for i, key := range keys {
		resBytes[i], _ = m.Get(key)
	}
	return resBytes
}

func (m *memoryBackend) Delete(key string) bool {
	exists := m.Exists(key)
	delete(m.client, key)
	return exists
}

func (m *memoryBackend) DeleteMany(keys []string) bool {
	var res bool
	for _, key := range keys {
		if m.Delete(key) {
			res = true
		}
	}
	return res
}

func (m *memoryBackend) Expire(key string, ttl time.Duration) bool {
	val, _ := m.Get(key)
	if val == nil {
		return false
	}
	m.client[key].ExpiredAt = time.Now().Add(ttl)
	return true
}

func (m *memoryBackend) TTL(key string) time.Duration {
	_, _ = m.Get(key)
	val := m.client[key]
	if val == nil {
		return 0
	}
	return time.Until(val.ExpiredAt)
}

func (m *memoryBackend) Exists(key string) bool {
	val, _ := m.Get(key)
	if val == nil {
		return false
	} else {
		return true
	}
}

func (m *memoryBackend) Close() error {
	return nil
}
