package redisext

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/shanbay/gobay"
	"go.elastic.co/apm/module/apmgoredis"
)

// RedisExt redis扩展，处理client的初始化工作
type RedisExt struct {
	NS          string
	app         *gobay.Application
	prefix      string
	redisclient *redis.Client

	apmEnable      bool
	apmredisclient apmgoredis.Client
}

var _ gobay.Extension = (*RedisExt)(nil)

// Init
func (c *RedisExt) Init(app *gobay.Application) error {
	if c.NS == "" {
		return errors.New("lack of NS")
	}
	c.app = app
	config := gobay.GetConfigByPrefix(app.Config(), c.NS, true)
	host := config.GetString("host")
	password := config.GetString("password")
	dbNum := config.GetInt("db")
	c.prefix = config.GetString("prefix")
	c.redisclient = redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       dbNum,
	})
	if app.Config().GetBool("elastic_apm_enable") {
		c.apmEnable = true
		c.apmredisclient = apmgoredis.Wrap(c.redisclient)
	}
	_, err := c.redisclient.Ping().Result()
	return err
}

func (c *RedisExt) CheckHealth(ctx context.Context) error {
	_, err := c.redisclient.Ping().Result()
	if err != nil {
		return err
	}

	cacheKey := c.prefix + "&GobayRedisExtensionHealthCheck&" + string(time.Now().Local().UnixNano())
	cacheValue := string(rand.Int63())
	err = c.Client(ctx).Set(cacheKey, cacheValue, 10*time.Second).Err()
	if err != nil {
		return err
	}
	gotValue, err := c.Client(ctx).Get(cacheKey).Result()
	if err != nil {
		return err
	}
	if gotValue != cacheValue {
		return fmt.Errorf("redis healthcheck cache result not match, expect %v, got %v", cacheValue, gotValue)
	}

	// test delete cache
	c.Client(ctx).Del(cacheKey)

	return nil
}

// Object return redis client
func (c *RedisExt) Object() interface{} {
	return c
}

// AddPrefix add prefix to a key
func (c *RedisExt) AddPrefix(key string) string {
	if c.prefix == "" {
		return key
	}
	return strings.Join([]string{c.prefix, key}, ".")
}

// Close close redis client
func (c *RedisExt) Close() error {
	return c.redisclient.Close()
}

// Application
func (c *RedisExt) Application() *gobay.Application {
	return c.app
}

func (c *RedisExt) Client(ctx context.Context) *redis.Client {
	if c.apmEnable {
		return c.apmredisclient.WithContext(ctx).RedisClient()
	}
	return c.redisclient.WithContext(ctx)
}
