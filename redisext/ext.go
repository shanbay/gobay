package redisext

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

var _ gobay.Extension = (*RedisExt)(nil)

// Init
func (c *RedisExt) Init(app *gobay.Application) error {
	c.app = app
	config := app.Config()
	if c.NS != "" {
		config = config.Sub(c.NS)
	}
	host := config.GetString("redis_host")
	password := config.GetString("redis_password")
	dbNum := config.GetInt("redis_db")
	c.client = redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       dbNum,
	})
	_, err := c.client.Ping().Result()
	return err
}

// Object return redis client
func (c *RedisExt) Object() interface{} {
	return c.client
}

// Close close redis client
func (c *RedisExt) Close() error {
	return c.client.Close()
}

// Application
func (c *RedisExt) Application() *gobay.Application {
	return c.app
}
