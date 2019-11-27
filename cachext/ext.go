package cachext

import (
	"bytes"
	"errors"
	"github.com/shanbay/gobay"
	"github.com/spf13/viper"
	"github.com/vmihailenco/msgpack"
	"sync"
	"time"
)

// CacheExt
type CacheExt struct {
	NS          string
	app         *gobay.Application
	backend     CacheBackend
	prefix      string
	mu          sync.Mutex
	initialized bool
}

var (
	_          gobay.Extension = (*CacheExt)(nil)
	backendMap                 = map[string]CacheBackend{}
	mu         sync.Mutex
)

type CacheBackend interface {
	Init(*viper.Viper) error
	Get(key string) ([]byte, error) // if record not exist, return (nil, nil)
	Set(key string, value []byte, ttl time.Duration) error
	SetMany(keyValues map[string][]byte, ttl time.Duration) error
	GetMany(keys []string) [][]byte // if record not exist, use nil instead
	Delete(key string) bool
	DeleteMany(keys []string) bool
	Expire(key string, ttl time.Duration) bool
	TTL(key string) time.Duration
	Exists(key string) bool
	Close() error
}

// Init init a cache extension
func (c *CacheExt) Init(app *gobay.Application) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}
	c.app = app
	config := app.Config()
	if c.NS != "" {
		config = config.Sub(c.NS)
		config.SetEnvPrefix(c.NS)
	}
	config.AutomaticEnv()
	c.prefix = config.GetString("cache_prefix")
	backendConfig := config.GetString("cache_backend")
	if backend, exist := backendMap[backendConfig]; exist {
		c.backend = backend
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
func RegisteBackend(configBackend string, backend CacheBackend) error {
	mu.Lock()
	defer mu.Unlock()
	if _, exist := backendMap[configBackend]; !exist {
		backendMap[configBackend] = backend
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
func (c *CacheExt) Get(key string, m interface{}) (bool, error) {
	transedKey := c.transKey(key)
	data, err := c.backend.Get(transedKey)
	if err == Nil {
		return false, err
	}
	return true, decode(data, m)
}

// Set
func (c *CacheExt) Set(key string, value interface{}, ttl time.Duration) error {
	transedKey := c.transKey(key)
	encodedValue, err := encode(value)
	if err != nil {
		return err
	}
	return c.backend.Set(transedKey, encodedValue, ttl)
}

// SetMany
func (c *CacheExt) SetMany(keyValues map[string]interface{}, ttl time.Duration) error {
	transedMap := make(map[string][]byte)
	for key, value := range keyValues {
		if encodedValue, err := encode(value); err != nil {
			return err
		} else {
			transedMap[c.transKey(key)] = encodedValue
		}
	}
	return c.backend.SetMany(transedMap, ttl)
}

// GetMany out map[string]*someStruct
func (c *CacheExt) GetMany(out map[string]interface{}) error {
	transedKeys := []string{}
	transedKey2key := make(map[string]string)
	for key := range out {
		transedKey := c.transKey(key)
		transedKeys = append(transedKeys, transedKey)
		transedKey2key[transedKey] = key
	}
	for i, value := range c.backend.GetMany(transedKeys) {
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
func (c *CacheExt) Delete(key string) bool {
	return c.backend.Delete(c.transKey(key))
}

// DeleteMany
func (c *CacheExt) DeleteMany(keys ...string) bool {
	transedKeys := make([]string, len(keys))
	for i, key := range keys {
		transedKeys[i] = c.transKey(key)
	}
	return c.backend.DeleteMany(transedKeys)
}

// Expire
func (c *CacheExt) Expire(key string, ttl time.Duration) bool {
	return c.backend.Expire(c.transKey(key), ttl)
}

// TTL
func (c *CacheExt) TTL(key string) time.Duration {
	return c.backend.TTL(c.transKey(key))
}

// Exists
func (c *CacheExt) Exists(key string) bool {
	return c.backend.Exists(c.transKey(key))
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
