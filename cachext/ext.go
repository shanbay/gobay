package cachext

import (
	"bytes"
	"errors"
	"github.com/shanbay/gobay"
	"github.com/vmihailenco/msgpack"
	"time"
)

// CacheExt
type CacheExt struct {
	NS         string
	app        *gobay.Application
	backend    CacheBackend
	backendMap map[string]CacheBackend
	prefix     string
}

var _ gobay.Extension = (*CacheExt)(nil)

// Init init a cache extension
func (c *CacheExt) Init(app *gobay.Application) error {
	c.app = app
	config := app.Config()
	if c.NS != "" {
		config = config.Sub(c.NS)
	}
	c.prefix = config.GetString("cache_prefix")
	if c.backendMap == nil {
		c.backendMap = make(map[string]CacheBackend)
	}
	c.backendMap["redis"] = &redisBackend{}
	c.backendMap["memory"] = &memBackend{}
	backendStr := config.GetString("cache_backend")
	if c.backendMap[backendStr] != nil {
		c.backend = c.backendMap[backendStr]
		if err := c.backend.Init(app); err != nil {
			return err
		}
	} else {
		return errors.New("No backend found for cache_backend:" + backendStr)
	}
	return nil
}

// RegisteBackend if you want a new backend, use this func to registe your backend
// then load it by config
func (c *CacheExt) RegisteBackend(configBackend string, backend CacheBackend) {
	if c.backendMap == nil {
		c.backendMap = make(map[string]CacheBackend)
	}
	c.backendMap[configBackend] = backend
}

// Close
func (c *CacheExt) Close() error {
	return c.backend.Close()
}

// Object
func (d *CacheExt) Object() interface{} {
	return d
}

// Application
func (d *CacheExt) Application() *gobay.Application {
	return d.app
}

func (c *CacheExt) transKey(key string) string {
	return c.prefix + key
}

// Get
func (c *CacheExt) Get(key string, m interface{}) (bool, error) {
	transedKey := c.transKey(key)
	data, err := c.backend.Get(transedKey)
	if data == nil {
		return false, err
	}
	return true, c.decode(data, m)
}

func (c *CacheExt) encode(value interface{}) ([]byte, error) {
	jsonBytes, err := msgpack.Marshal(&value)
	if err != nil {
		return []byte{}, err
	}
	return jsonBytes, nil
}

func (c *CacheExt) decode(data interface{}, out interface{}) error {
	if bytesData, ok := data.([]byte); ok {
		return msgpack.Unmarshal(bytesData, out)
	} else if strData, ok := data.(string); ok {
		return msgpack.Unmarshal(([]byte)(strData), out)
	} else {
		return errors.New("Invalid param: out, can not set to it")
	}
}

func (c *CacheExt) decodeIsNil(data interface{}) bool {
	if bytesData, ok := data.([]byte); ok {
		err := msgpack.NewDecoder(bytes.NewReader(bytesData)).DecodeNil()
		return (err == nil)
	} else if strData, ok := data.(string); ok {
		err := msgpack.NewDecoder(bytes.NewReader(([]byte)(strData))).DecodeNil()
		return (err == nil)
	}
	return false
}

// Set
func (c *CacheExt) Set(key string, value interface{}, ttl int64) error {
	transedKey := c.transKey(key)
	encodedValue, err := c.encode(value)
	if err != nil {
		return err
	}
	return c.backend.Set(transedKey, encodedValue, time.Duration(ttl)*time.Second)
}

// SetMany
func (d *CacheExt) SetMany(keyValues map[string]interface{}, ttl int64) error {
	transedMap := make(map[string]interface{})
	for key, value := range keyValues {
		if encodedValue, err := d.encode(value); err != nil {
			return err
		} else {
			transedMap[d.transKey(key)] = encodedValue
		}
	}
	return d.backend.SetMany(transedMap, time.Duration(ttl)*time.Second)
}

// GetMany out map[string]*someStruct
func (d *CacheExt) GetMany(out map[string]interface{}) error {
	transedKeys := []string{}
	transedKey2key := make(map[string]string)
	for key, _ := range out {
		transedKey := d.transKey(key)
		transedKeys = append(transedKeys, transedKey)
		transedKey2key[transedKey] = key
	}
	for i, value := range d.backend.GetMany(transedKeys) {
		key := transedKey2key[transedKeys[i]]
		if value != nil {
			if err := d.decode(value, out[key]); err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete
func (d *CacheExt) Delete(key string) bool {
	return d.backend.Delete(d.transKey(key))
}

// DeleteMany
func (d *CacheExt) DeleteMany(keys ...string) bool {
	transedKeys := make([]string, len(keys))
	for i, key := range keys {
		transedKeys[i] = d.transKey(key)
	}
	return d.backend.DeleteMany(transedKeys)
}

// Expire
func (d *CacheExt) Expire(key string, ttl int) bool {
	return d.backend.Expire(d.transKey(key), time.Duration(ttl)*time.Second)
}

// TTL
func (d *CacheExt) TTL(key string) int64 {
	return d.backend.TTL(d.transKey(key))
}

// Exists
func (d *CacheExt) Exists(key string) bool {
	return d.backend.Exists(d.transKey(key))
}
