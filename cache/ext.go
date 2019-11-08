package cache

import (
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
	backend cacheBackend
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
	if backend == "memory" {
		// TODO
	} else {
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
	}
	return nil
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
func (c *CacheExt) CachedFunc(cache_none bool, ttl int64, version int, f interface{}) func(args ...interface{}) (interface{}, error) {

	return func(args ...interface{}) (interface{}, error) {
		cache_key, err := c.MakeCacheKey(f, version, args...)
		if err != nil {
			return nil, err
		}
		cache_res := c.Get(cache_key)
		if cache_res == nil {
			inputs := make([]reflect.Value, len(args))
			for i, _ := range args {
				inputs[i] = reflect.ValueOf(args[i])
			}
			cache_res = reflect.ValueOf(f).Call(inputs)
			call_res := cache_res.([]reflect.Value)[0].Interface()
			call_err, is_error := cache_res.([]reflect.Value)[1].Interface().(error)
			if call_err != nil && is_error {
				return nil, call_err
			}
			if call_res == nil && cache_none == true {
				err := c.Set(cache_key, new(cacheResNone), ttl)
				if err != nil {
				}
				return nil, nil
			} else {
				err := c.Set(cache_key, call_res, ttl)
				if err != nil {
				}
				return call_res, nil
			}
		} else {
			if cache_none == true && cache_res == *new(cacheResNone) {
				return nil, nil
			} else {
				return cache_res, nil
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

// Get 获取某个缓存key是否存在
func (d *CacheExt) Get(key string) interface{} {
	transed_key := d.trans_key(key)
	return d.backend.Get(transed_key)
}

const _TTL_UNIT = 1000000 // time.Duration的单位是ns，这里转换成秒

// Set 设置某个缓存值，设置时必须要填写一个ttl，如果想要使用nx=True这样
// 的参数，可以使用redis实例。
func (d *CacheExt) Set(key string, value interface{}, ttl int64) error {
	transed_key := d.trans_key(key)
	return d.backend.Set(transed_key, value, time.Duration(ttl)*time.Second)
}

// SetMany MSet命令，会重置所有key的过期时间.
func (d *CacheExt) SetMany(keys []string, values []interface{}, ttl int64) error {
	transed_keys := make([]string, len(keys))
	for i, key := range keys {
		transed_keys[i] = d.trans_key(key)
	}
	return d.backend.SetMany(transed_keys, values, time.Duration(ttl)*time.Second)
}

// GetMany
func (d *CacheExt) GetMany(keys []string) []interface{} {
	transed_keys := make([]string, len(keys))
	for i, key := range keys {
		transed_keys[i] = d.trans_key(key)
	}
	return d.backend.GetMany(transed_keys)
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
