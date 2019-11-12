package cache

import (
	"encoding/json"
	"errors"
	"github.com/go-redis/redis"
	"github.com/shanbay/gobay"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// CacheExt 缓存扩展，提供了方便的缓存操作，可以选择backend
// 目前支持的backend有内存、redis。可以配置前缀，避免多个项目
// 共用一个redis实例时发生冲突。
type CacheExt struct {
	// gobay.Extension
	NS      string
	app     *gobay.Application
	backend CacheBackend
	prefix  string
}

// Init init a cache extension
func (c *CacheExt) Init(app *gobay.Application) error {
	c.app = app
	config := app.Config()
	if c.NS != "" {
		config = config.Sub(c.NS)
	}
	backend := config.GetString("cache_backend")
	if backend == "redis" {
		// redis backend
		c.prefix = config.GetString("cache_prefix")
		host := config.GetString("cache_host")
		password := config.GetString("cache_password")
		db_num := config.GetInt("cache_db")
		redisClient := redis.NewClient(&redis.Options{
			Addr:     host,
			Password: password,
			DB:       db_num,
		})
		_, err := redisClient.Ping().Result()
		if err != nil {
			return err
		}
		redisBack := new(redisBackend)
		redisBack.SetClient(redisClient)
		c.backend = redisBack

	} else {
		c.backend = new(memBackend)
		client := make(map[string]interface{})
		c.backend.SetClient(client)
	}
	return nil
}

// SetBackend 如果调用方想要自己定义backend，可以由这个方法设置进来
func (c *CacheExt) SetBackend(backend CacheBackend) {
	c.backend = backend
}

// MakeCacheKey 用于生成函数的缓存key，带版本控制。只允许数字、布尔、字符串这几种类似的参数。
// 使用#号拼接各个参数，尽量不要在字符串中出现#以避免碰撞
func (c *CacheExt) MakeCacheKey(f interface{}, version int, args ...interface{}) (string, error) {
	inputs := make([]string, len(args)+2)
	inputs[0] = runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	inputs[1] = strconv.Itoa(version)
	for i, _ := range args {
		v := reflect.ValueOf(args[i])
		i += 2
		switch v.Kind() {
		case reflect.Invalid:
			return "", errors.New("invalid")
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			inputs[i] = strconv.FormatInt(v.Int(), 10)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			inputs[i] = strconv.FormatUint(v.Uint(), 10)
		case reflect.Bool:
			inputs[i] = strconv.FormatBool(v.Bool())
		case reflect.String:
			inputs[i] = v.String()
		default:
			return "", errors.New("Unsupported args type: " + v.Type().String())
		}
	}
	return strings.Join(inputs, "#"), nil
}

type cacheResNone struct{}

// CachedFunc 把一个正常函数变成一个缓存函数，类似python里的装饰器
// 缓存的key就是MakeCacheKey生成的key
func (c *CacheExt) CachedFunc(f interface{}, cache_none bool, ttl int64, version int) func(args ...interface{}) (interface{}, error) {
	real_func := reflect.TypeOf(f)

	return func(args ...interface{}) (interface{}, error) {
		cache_key, err := c.MakeCacheKey(f, version, args...)
		if err != nil {
			return nil, err
		}
		serialize, err := c.get(cache_key)
		if err != nil {
			return nil, err
		}
		// 决策是否调用函数
		reCall := false
		if serialize == nil {
			reCall = true
		} else if cache_none == false && serialize.Data == nil {
			// 缓存里保存了一个空值，但是本次调用不允许缓存空值
			reCall = true
		}
		if reCall == true {
			inputs := make([]reflect.Value, len(args))
			for i, _ := range args {
				inputs[i] = reflect.ValueOf(args[i])
			}
			var cache_res interface{} = reflect.ValueOf(f).Call(inputs)
			call_res := cache_res.([]reflect.Value)[0].Interface()
			call_err, is_error := cache_res.([]reflect.Value)[1].Interface().(error)
			if is_error {
				return call_res, call_err
			}
			if call_res == nil {
				if cache_none == true {
					err := c.set(cache_key, nil, ttl)
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			} else {
				err := c.set(cache_key, call_res, ttl)
				if err != nil {
					return call_res, err
				}
				return call_res, nil
			}
		} else {
			if cache_none == true && serialize.Data == nil {
				return nil, nil
			} else {
				return serialize.Data, nil
			}
		}
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

type serializeData struct {
	Data interface{}
}

func (c *CacheExt) toGob64(data interface{}) (string, error) {
	serialize := new(serializeData)
	serialize.Data = data
	json_str, err := json.Marshal(serialize)
	if err != nil {
		return "", err
	}
	return string(json_str), nil
}

func (c *CacheExt) fromGob64(data string) (interface{}, error) {
	m := new(serializeData)
	err := json.Unmarshal([]byte(data), m)
	if err != nil {
		return nil, err
	}
	return m.Data, nil
}

// Get 获取某个缓存key是否存在
func (c *CacheExt) Get(key string) (interface{}, error) {
	serialize, err := c.get(key)
	if err != nil {
		return nil, err
	}
	return serialize.Data, nil
}

func (c *CacheExt) get(key string) (*serializeData, error) {
	transed_key := c.trans_key(key)
	data := c.backend.Get(transed_key)
	if data == nil {
		return nil, nil
	}
	str_data, ok := data.(string)
	if ok == false {
		return nil, errors.New("trans to string failed:" + str_data)
	}
	m := new(serializeData)
	err := json.Unmarshal([]byte(str_data), m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Set 设置某个缓存值，设置时必须要填写一个ttl，如果想要使用nx=True这样
// 的参数，可以使用redis实例。
func (c *CacheExt) Set(key string, value interface{}, ttl int64) error {
	if value == nil {
		return errors.New("Not allowed to set nil")
	}
	return c.set(key, value, ttl)
}

func (c *CacheExt) set(key string, value interface{}, ttl int64) error {
	transed_key := c.trans_key(key)
	data, err := c.toGob64(value)
	if err != nil {
		return err
	}
	return c.backend.Set(transed_key, data, time.Duration(ttl)*time.Second)
}

// SetMany MSet命令，会重置所有key的过期时间.
func (d *CacheExt) SetMany(keys []string, values []interface{}, ttl int64) error {
	transed_keys := make([]string, len(keys))
	transed_values := make([]string, len(values))
	for i, key := range keys {
		transed_keys[i] = d.trans_key(key)
		data, err := d.toGob64(values[i])
		if err != nil {
			return err
		}
		transed_values[i] = data
	}
	return d.backend.SetMany(transed_keys, transed_values, time.Duration(ttl)*time.Second)
}

// GetMany
func (d *CacheExt) GetMany(keys []string) []interface{} {
	transed_keys := make([]string, len(keys))
	for i, key := range keys {
		transed_keys[i] = d.trans_key(key)
	}
	res := make([]interface{}, len(keys))
	for i, val := range d.backend.GetMany(transed_keys) {
		if val == nil {
			res[i] = nil
		} else {
			data, _ := d.fromGob64(val.(string))
			res[i] = data
		}
	}
	return res
}

// Delete
func (d *CacheExt) Delete(key string) int64 {
	return d.backend.Delete(d.trans_key(key))
}

func (d *CacheExt) DeleteMany(keys []string) int64 {
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

// Clear
func (d *CacheExt) Clear() string {
	return d.backend.Clear()
}
