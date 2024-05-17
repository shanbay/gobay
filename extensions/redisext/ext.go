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
)

// RedisExt redis扩展，处理client的初始化工作
type RedisExt struct {
	NS          string
	app         *gobay.Application
	prefix      string
	redisclient *redis.Client
}

var _ gobay.Extension = (*RedisExt)(nil)

// Init
func (c *RedisExt) Init(app *gobay.Application) error {
	if c.NS == "" {
		return errors.New("lack of NS")
	}
	c.app = app
	config := gobay.GetConfigByPrefix(app.Config(), c.NS, true)
	opt := redis.Options{}
	if err := config.Unmarshal(&opt); err != nil {
		return err
	}
	c.prefix = config.GetString("prefix")
	c.redisclient = redis.NewClient(&opt)
	_, err := c.redisclient.Ping().Result()
	return err
}

func (c *RedisExt) CheckHealth(ctx context.Context) error {
	_, err := c.redisclient.Ping().Result()
	if err != nil {
		return err
	}

	cacheKey := c.prefix + "&GobayRedisExtensionHealthCheck&" + fmt.Sprint(time.Now().Local().UnixNano())
	cacheValue := fmt.Sprint(rand.Int63())
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
	return c.redisclient.WithContext(ctx)
}

func (c *RedisExt) EvalLua(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	cmd := c.Client(ctx).Eval(script, keys, args...)
	return cmd.Result()
}
