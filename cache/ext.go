package cache

import (
	"github.com/go-redis/redis"
	"github.com/shanbay/gobay"
	"time"
)

// RedisExt redis扩展，处理client的初始化工作
type RedisExt struct {
	NS     string
	app    *gobay.Application
	client *redis.Client
}

// Init
func (d *RedisExt) Init(app *gobay.Application) error {
	host := config.GetString("redis_host")
	password := config.GetString("redis_password")
	dbNum := config.GetUint32("redis_db")
	d.client = redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       dbNum,
	})
	pong, err = d.client.Ping().Result()
	if err != nil {
		fmt.Println(pong, err)
	}
}

// Object return redis client
func (d *RedisExt) Object() *redis.Client {
	return d.client
}

// Close close redis client
func (d *RedisExt) Close() error {
	return d.client.Close()
}

// CacheExt 缓存扩展，提供了方便的缓存操作，可以选择backend
// 目前支持的backend有内存、redis。可以配置前缀，避免多个项目
// 共用一个redis实例时发生冲突。
type CacheExt struct {
	NS     string
	app    *gobay.Application
	client *redis.Client
	prefix string
}

// Init init a cache extension
func (c *CacheExt) Init(app *gobay.Application) error {
	c.app = app
	config := app.Config()
	if d.NS != "" {
		config = config.Sub(c.NS)
	}
	c.prefix = config.GetString("cache_prefix")
	host := config.GetString("cache_host")
	password := config.GetString("cache_password")
	db_num := config.GetUint32("cache_db")
	c.client = redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       db_num,
	})
	pong, err = c.client.Ping().Result()
	if err != nil {
		fmt.Println(pong, err)
	}
}

// Close
func (c *CacheExt) Close() error {
	return c.client.Close()
}

func (c *CacheExt) trans_key(key string) string {
	return c.prefix + key
}

// Get 获取某个缓存key是否存在
func (d *CacheExt) Get(key string) interface{} {
	transed_key := d.trans_key(key)
	return d.client.Get(transed_key).Result()
}

// Set 设置某个缓存值，设置时必须要填写一个ttl，如果想要使用nx=True这样
// 的参数，可以使用redis实例。
func (d *CacheExt) Set(key string, value interface{}, ttl int64) {
	transed_key := d.trans_key(key)
	return d.client.Set(transed_key, value, time.Duration(ttl)).Err()
}

// SetMany MSet命令，会重置所有key的过期时间.
func (d *CacheExt) SetMany(keys []string, values []interface{}, ttl int64) {
	transed_keys := make([]string, len(keys))
	pairs := make([]interface{}, 2*len(keys))
	for i, _ := range keys {
		var transed_key = d.trans_key(keys[i])
		pairs = append(pairs, transed_key, values[i])
		transed_keys = append(transed_keys, transed_key)
	}
	d.client.MSet(pairs...)
	for i, _ := range transed_keys {
		d.client.Expire(transed_keys[i], time.Duration(ttl))
	}
}

// GetMany
func (d *CacheExt) GetMany(keys []string) []interface{} {
	pairs := make([]string, len(keys))
	for i, key := range keys {
		pairs[i] = d.trans_key(key)
	}
	res, err := d.MGet(pairs...)
	if err != nil {
		return err
	}
	return res.val
}

// Delete
func (d *CacheExt) Delete(key string) int64 {
	keys = make([]string, 1)
	keys[0] = key
	return d.DeleteMany(keys)
}

func (d *CacheExt) DeleteMany(keys []string) int64 {
	transed_keys := make([]string, len(keys))
	for i, key := range keys {
		transed_keys[i] = d.trans_key(key)
	}
	res := d.client.Del(transed_keys...)
	return res.val
}

// Expire
func (d *CacheExt) Expire(key string, ttl int) int64 {
	transed_key := d.trans_key(key)
	res := d.client.Expire(transed_key, time.Duration(ttl))
	return res.val
}

// TTL
func (d *CacheExt) TTL(key string) int64 {
	transed_key := d.trans_key(key)
	res := d.client.TTL(transed_key)
	return int64(res.val)
}

// Exists
func (d *CacheExt) Exists(key string) bool {
	keys = make([]string, 1)
	keys[0] = d.trans_key(key)
	res := d.client.Exists(keys...)
	var exists bool
	exists = (res.val == 1)
	return exists
}

// Clear
func (d *CacheExt) Clear() string {
	res := d.client.FlushDB()
	return res.val
}
