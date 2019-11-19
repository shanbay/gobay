package cachext

import (
	"errors"
	"github.com/shanbay/gobay"
	"github.com/vmihailenco/msgpack"
	"net/url"
	"strconv"
	"strings"
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

var _ gobay.Extension = &CacheExt{}

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
	backend_str := config.GetString("cache_backend")
	if c.backendMap[backend_str] != nil {
		c.backend = c.backendMap[backend_str]
		if err := c.backend.Init(app); err != nil {
			return err
		}
	} else {
		return errors.New("No backend found, config cache_backend:" + backend_str)
	}
	return nil
}

// RegisteBackend if you want a new backend, use this func to registe your backend
// then load it by config
func (c *CacheExt) RegisteBackend(config_backend string, backend CacheBackend) {
	if c.backendMap == nil {
		c.backendMap = make(map[string]CacheBackend)
	}
	c.backendMap[config_backend] = backend
}

// MakeCacheKey
func (c *CacheExt) MakeCacheKey(funcName string, arg1 []string, arg2 ...int) string {
	inputs := make([]string, len(arg1)+len(arg2)+1)
	inputs[0] = funcName
	for i, arg := range arg1 {
		i += 1
		inputs[i] = arg
	}
	for i, arg := range arg2 {
		i += 1 + len(arg1)
		inputs[i] = strconv.Itoa(arg)
	}
	for i, input := range inputs {
		inputs[i] = url.QueryEscape(input)
	}
	return strings.Join(inputs, "&")
}

// Cached return another cached func
func (c *CacheExt) Cached(funcName string, ttl int64, f func([]string, ...int) (interface{}, error)) func(interface{}, []string, ...int) error {
	return func(result interface{}, arg1 []string, arg2 ...int) error {
		cacheKey := c.MakeCacheKey(funcName, arg1, arg2...)
		if exist, decode_err := c.Get(cacheKey, result); decode_err != nil {
			return decode_err
		} else if !exist {
			value, err := f(arg1, arg2...)
			if err != nil {
				return err
			}
			if err = c.Set(cacheKey, value, ttl); err != nil {
				return err
			}
			encode_str, _ := c.encode(value)
			c.decode(encode_str, result)
		}
		return nil
	}
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

func (c *CacheExt) trans_key(key string) string {
	return c.prefix + key
}

// Get
func (c *CacheExt) Get(key string, m interface{}) (bool, error) {
	transed_key := c.trans_key(key)
	data, err := c.backend.Get(transed_key)
	if data == nil {
		return false, err
	}
	return true, c.decode(data, m)
}

func (c *CacheExt) encode(value interface{}) ([]byte, error) {
	json_bytes, err := msgpack.Marshal(&value)
	if err != nil {
		return []byte{}, err
	}
	return json_bytes, nil
}

func (c *CacheExt) decode(data interface{}, out interface{}) error {
	if strData, ok := data.([]byte); ok == true {
		return msgpack.Unmarshal(strData, out)
	} else {
		return errors.New("Invalid param: out, can not set to it")
	}
}

// Set
func (c *CacheExt) Set(key string, value interface{}, ttl int64) error {
	transed_key := c.trans_key(key)
	encoded_value, err := c.encode(value)
	if err != nil {
		return err
	}
	return c.backend.Set(transed_key, encoded_value, time.Duration(ttl)*time.Second)
}

// SetMany
func (d *CacheExt) SetMany(keyValues map[string]interface{}, ttl int64) error {
	transed_map := make(map[string]interface{})
	for key, value := range keyValues {
		if encodedValue, err := d.encode(value); err != nil {
			return err
		} else {
			transed_map[d.trans_key(key)] = encodedValue
		}
	}
	return d.backend.SetMany(transed_map, time.Duration(ttl)*time.Second)
}

// GetMany out map[string]*someStruct
func (d *CacheExt) GetMany(out map[string]interface{}) error {
	transed_keys := []string{}
	transed_key_key := make(map[string]string)
	for key, _ := range out {
		transed_key := d.trans_key(key)
		transed_keys = append(transed_keys, transed_key)
		transed_key_key[transed_key] = key
	}
	for i, value := range d.backend.GetMany(transed_keys) {
		key := transed_key_key[transed_keys[i]]
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
	return d.backend.Delete(d.trans_key(key))
}

// DeleteMany
func (d *CacheExt) DeleteMany(keys ...string) bool {
	transed_keys := make([]string, len(keys))
	for i, key := range keys {
		transed_keys[i] = d.trans_key(key)
	}
	return d.backend.DeleteMany(transed_keys)
}

// Expire
func (d *CacheExt) Expire(key string, ttl int) bool {
	return d.backend.Expire(d.trans_key(key), time.Duration(ttl)*time.Second)
}

// TTL
func (d *CacheExt) TTL(key string) int64 {
	return d.backend.TTL(d.trans_key(key))
}

// Exists
func (d *CacheExt) Exists(key string) bool {
	return d.backend.Exists(d.trans_key(key))
}
