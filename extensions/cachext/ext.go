package cachext

import (
	"bytes"
	"context"
	"errors"
	"github.com/shanbay/gobay"
	"github.com/spf13/viper"
	"github.com/vmihailenco/msgpack"
	"sync"
	"time"
)

type void struct{}

// CacheExt
type CacheExt struct {
	NS             string
	app            *gobay.Application
	backend        CacheBackend
	prefix         string
	initialized    bool
	cachedFuncName map[string]void
}

var (
	_          gobay.Extension = (*CacheExt)(nil)
	backendMap                 = map[string](func() CacheBackend){}
	mu         sync.Mutex
)

type CacheBackend interface {
	Init(*viper.Viper) error
	Get(context.Context, string) ([]byte, error) // if record not exist, return (nil, nil)
	Set(context.Context, string, []byte, time.Duration) error
	SetMany(context.Context, map[string][]byte, time.Duration) error
	GetMany(context.Context, []string) [][]byte // if record not exist, use nil instead
	Delete(context.Context, string) bool
	DeleteMany(context.Context, []string) bool
	Expire(context.Context, string, time.Duration) bool
	TTL(context.Context, string) time.Duration
	Exists(context.Context, string) bool
	Close() error
}

// Init init a cache extension
func (c *CacheExt) Init(app *gobay.Application) error {
	if c.NS == "" {
		return errors.New("cachext: lack of NS")
	}
	mu.Lock()
	defer mu.Unlock()

	if c.initialized {
		return nil
	}
	c.app = app
	c.cachedFuncName = make(map[string]void)
	config := app.Config()
	config = gobay.GetConfigByPrefix(config, c.NS, true)
	c.prefix = config.GetString("prefix")
	backendConfig := config.GetString("backend")
	if backendFunc, exist := backendMap[backendConfig]; exist {
		c.backend = backendFunc()
		if err := c.backend.Init(config); err != nil {
			return err
		}
	} else {
		return errors.New("No backend found for cache_backend:" + backendConfig)
	}
	c.initialized = true
	return nil
}

// RegisteBackend if you want a new backend, use this func to registe your backend
// then load it by config
func RegisteBackend(configBackend string, backendFunc func() CacheBackend) error {
	mu.Lock()
	defer mu.Unlock()
	if _, exist := backendMap[configBackend]; !exist {
		backendMap[configBackend] = backendFunc
		return nil
	} else {
		return errors.New("Backend already registered")
	}
}

// Close
func (c *CacheExt) Close() error {
	return c.backend.Close()
}

// Object
func (c *CacheExt) Object() interface{} {
	return c
}

// Application
func (c *CacheExt) Application() *gobay.Application {
	return c.app
}

func (c *CacheExt) transKey(key string) string {
	return c.prefix + key
}

// Get
func (c *CacheExt) Get(ctx context.Context, key string, m interface{}) (bool, error) {
	transedKey := c.transKey(key)
	data, err := c.backend.Get(ctx, transedKey)
	if data == nil {
		return false, err
	}
	return true, decode(data, m)
}

// Set
func (c *CacheExt) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	transedKey := c.transKey(key)
	encodedValue, err := encode(value)
	if err != nil {
		return err
	}
	return c.backend.Set(ctx, transedKey, encodedValue, ttl)
}

// SetMany
func (c *CacheExt) SetMany(ctx context.Context, keyValues map[string]interface{}, ttl time.Duration) error {
	transedMap := make(map[string][]byte)
	for key, value := range keyValues {
		if encodedValue, err := encode(value); err != nil {
			return err
		} else {
			transedMap[c.transKey(key)] = encodedValue
		}
	}
	return c.backend.SetMany(ctx, transedMap, ttl)
}

// GetMany out map[string]*someStruct
func (c *CacheExt) GetMany(ctx context.Context, out map[string]interface{}) error {
	transedKeys := []string{}
	transedKey2key := make(map[string]string)
	for key := range out {
		transedKey := c.transKey(key)
		transedKeys = append(transedKeys, transedKey)
		transedKey2key[transedKey] = key
	}
	for i, value := range c.backend.GetMany(ctx, transedKeys) {
		key := transedKey2key[transedKeys[i]]
		if value != nil {
			if err := decode(value, out[key]); err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete
func (c *CacheExt) Delete(ctx context.Context, key string) bool {
	return c.backend.Delete(ctx, c.transKey(key))
}

// DeleteMany
func (c *CacheExt) DeleteMany(ctx context.Context, keys ...string) bool {
	transedKeys := make([]string, len(keys))
	for i, key := range keys {
		transedKeys[i] = c.transKey(key)
	}
	return c.backend.DeleteMany(ctx, transedKeys)
}

// Expire
func (c *CacheExt) Expire(ctx context.Context, key string, ttl time.Duration) bool {
	return c.backend.Expire(ctx, c.transKey(key), ttl)
}

// TTL
func (c *CacheExt) TTL(ctx context.Context, key string) time.Duration {
	return c.backend.TTL(ctx, c.transKey(key))
}

// Exists
func (c *CacheExt) Exists(ctx context.Context, key string) bool {
	return c.backend.Exists(ctx, c.transKey(key))
}

func encode(value interface{}) ([]byte, error) {
	jsonBytes, err := msgpack.Marshal(&value)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

func decode(data []byte, out interface{}) error {
	return msgpack.Unmarshal(data, out)
}

func decodeIsNil(data interface{}) bool {
	if byteData, ok := data.([]byte); ok {
		err := msgpack.NewDecoder(bytes.NewReader(byteData)).DecodeNil()
		return (err == nil)
	}
	return false
}
