package cache

import (
	"encoding/json"
	"errors"
	"github.com/go-redis/redis"
	"github.com/shanbay/gobay"
	"net/url"
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
		c.backend = &redisBackend{client: redisClient}

	} else {
		c.backend = &memBackend{client: make(map[string]interface{}), ttl: make(map[string]time.Duration)}
	}
	return nil
}

// SetBackend 如果调用方想要自己定义backend，可以由这个方法设置进来
func (c *CacheExt) SetBackend(backend CacheBackend) {
	c.backend = backend
}

// MakeCacheKey 用于生成函数的缓存key，带版本控制。只允许数字、布尔、字符串这几种类似的参数。
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
	for i, input := range inputs {
		inputs[i] = url.QueryEscape(input)
	}
	return strings.Join(inputs, "&"), nil
}

// CachedFunc 把一个正常函数变成一个缓存函数，类似python里的装饰器
// 缓存的key就是MakeCacheKey生成的key
// 对函数的要求：返回值有两个，第二个返回值是error 第一个返回值是interface{}且返回值是nil会导致无法缓存，
// 如果想要cache_none请返回零值 函数参数中不可以出现 ...参数
// 第一个返回值可以是基本类型：数字，字符串，布尔，也可以是结构体。不支持指针作为返回值, 不推荐使用interface{}作为返回值
func (c *CacheExt) CachedFunc(function interface{}, ttl int64, version int) (func(args ...interface{}) (interface{}, error), error) {
	fValue := reflect.ValueOf(function)
	if fValue.Kind() != reflect.Func {
		return func(args ...interface{}) (interface{}, error) {
			return nil, nil
		}, errors.New("Generate func failed, the first param must be a function!")
	}
	if fValue.Type().NumOut() != 2 || !fValue.Type().Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return func(args ...interface{}) (interface{}, error) {
			return nil, nil
		}, errors.New("Generate func failed, the response of function is invalid, should return (SomeStruct, error)")
	}
	return func(args ...interface{}) (interface{}, error) {
		cacheKey, err := c.MakeCacheKey(function, version, args...)
		if err != nil {
			return nil, err
		}
		fOut := fValue.Type().Out(0)
		var cacheRes interface{}
		var reCall bool
		cacheRes = reflect.New(fOut).Interface()
		var exist bool
		exist, err = c.Get(cacheKey, cacheRes)
		if err != nil {
			return nil, err
		}
		if exist == false {
			reCall = true
		} else {
			// 命中缓存，把返回值指针转换成返回值结构体
			cacheRes = reflect.ValueOf(cacheRes).Elem().Interface()
		}
		if reCall { // 重新构建缓存
			inputs := make([]reflect.Value, len(args))
			for i, _ := range args {
				inputs[i] = reflect.ValueOf(args[i])
			}
			var outputs interface{} = fValue.Call(inputs)
			var callRes interface{} = outputs.([]reflect.Value)[0].Interface()
			if callErr, isError := outputs.([]reflect.Value)[1].Interface().(error); isError {
				return callRes, callErr
			}
			cacheRes = callRes
			if callRes != nil {
				// 重置缓存
				err = c.Set(cacheKey, callRes, ttl)
				if err != nil {
					return callRes, err
				}
			}
		}
		return cacheRes, nil
	}, nil
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

// Get  Get("hello", &someStruce)
func (c *CacheExt) Get(key string, m interface{}) (bool, error) {
	if reflect.ValueOf(m).Kind() != reflect.Ptr {
		return false, errors.New("Invalid param: m, want a struct's ptr")
	}
	transed_key := c.trans_key(key)
	data, err := c.backend.Get(transed_key)
	if data == nil {
		return false, err
	}
	if _, ok := data.([]byte); ok == true {
		err = json.Unmarshal(data.([]byte), m)
		return true, err
	} else {
		mValue := reflect.ValueOf(m).Elem()
		if mValue.CanSet() == false {
			return true, errors.New("Invalid param: m, can not set to it")
		}
		mValue.Set(reflect.ValueOf(data))
		return true, nil
	}
}

func (c *CacheExt) validType(t reflect.Type) error {
	isSlice, err := c.validKind(t.Kind())
	if isSlice {
		return c.validType(t.Elem())
	}
	return err
}

func (c *CacheExt) validKind(kind reflect.Kind) (bool, error) {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
	case reflect.Bool:
	case reflect.String:
	case reflect.Struct:
	case reflect.Slice, reflect.Array, reflect.Map:
		return true, nil
	default:
		return false, errors.New("Basic value not supported type: " + kind.String())
	}
	return false, nil
}

// Set
func (c *CacheExt) Set(key string, value interface{}, ttl int64) error {
	if err := c.validType(reflect.TypeOf(value)); err != nil {
		return err
	}
	transed_key := c.trans_key(key)
	json_bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.backend.Set(transed_key, json_bytes, time.Duration(ttl)*time.Second)
}

// SetMany MSet命令，会重置所有key的过期时间.
func (d *CacheExt) SetMany(keyValues map[string]interface{}, ttl int64) error {
	transed_map := make(map[string]interface{})
	for key, value := range keyValues {
		if err := d.validType(reflect.TypeOf(value)); err != nil {
			return err
		}
		transed_map[d.trans_key(key)] = value
	}
	return d.backend.SetMany(keyValues, time.Duration(ttl)*time.Second)
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
