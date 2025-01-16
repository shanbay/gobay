package redisv9ext

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/shanbay/gobay/observability"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/shanbay/gobay"
	"go.opentelemetry.io/otel"
)

// RedisExt redis扩展，处理client的初始化工作
type RedisExt struct {
	NS          string
	app         *gobay.Application
	prefix      string
	redisClient *redis.Client
}

var _ gobay.Extension = (*RedisExt)(nil)

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
	c.redisClient = redis.NewClient(&opt)
	if observability.GetOtelEnable() {
		tp := otel.GetTracerProvider()
		if err := redisotel.InstrumentTracing(c.redisClient, redisotel.WithTracerProvider(tp)); err != nil {
			return err
		}
	}
	_, err := c.redisClient.Ping(context.Background()).Result()
	return err
}

func (c *RedisExt) CheckHealth(ctx context.Context) error {
	_, err := c.redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}

	cacheKey := c.prefix + "&GobayRedisExtensionHealthCheck&" + fmt.Sprint(time.Now().Local().UnixNano())
	cacheValue := fmt.Sprint(rand.Int63())
	err = c.redisClient.Set(ctx, cacheKey, cacheValue, 10*time.Second).Err()
	if err != nil {
		return err
	}
	gotValue, err := c.redisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		return err
	}
	if gotValue != cacheValue {
		return fmt.Errorf("redis healthcheck cache result not match, expect %v, got %v", cacheValue, gotValue)
	}

	// test delete cache
	c.redisClient.Del(ctx, cacheKey)

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
	return c.redisClient.Close()
}

// Application
func (c *RedisExt) Application() *gobay.Application {
	return c.app
}

func (c *RedisExt) Client() *redis.Client {
	return c.redisClient
}

func (c *RedisExt) EvalLua(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	cmd := c.redisClient.Eval(ctx, script, keys, args...)
	return cmd.Result()
}
