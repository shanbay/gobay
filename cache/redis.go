package cache

import (
	"github.com/go-redis/redis"
	"github.com/shanbay/gobay"
)

// RedisExt redis扩展，处理client的初始化工作
type RedisExt struct {
	NS     string
	app    *gobay.Application
	client *redis.Client
}

var _ gobay.Extension = &RedisExt{}

// Init
func (d *RedisExt) Init(app *gobay.Application) error {
	d.app = app
	config := app.Config()
	host := config.GetString("redis_host")
	password := config.GetString("redis_password")
	dbNum := config.GetInt("redis_db")
	d.client = redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       dbNum,
	})
	_, err := d.client.Ping().Result()
	return err
}

// Object return redis client
func (d *RedisExt) Object() interface{} {
	return d.client
}

// Close close redis client
func (d *RedisExt) Close() error {
	return d.client.Close()
}

// Application
func (d *RedisExt) Application() *gobay.Application {
	return d.app
}
