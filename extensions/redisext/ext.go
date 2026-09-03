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
	"github.com/shanbay/gobay/observability"
	"go.elastic.co/apm/module/apmgoredis"
)

// RedisExt redis扩展，处理client的初始化工作
type RedisExt struct {
	NS             string
	app            *gobay.Application
	prefix         string
	redisclient    *redis.Client
	apmable        bool
	apmredisclient apmgoredis.Client
}

var _ gobay.Extension = (*RedisExt)(nil)

// go-redis 默认的 PoolSize 是 10*runtime.NumCPU()，NumCPU 读的是宿主机核数、不感知
// 容器的 CPU limit，多核节点上的容器会拿到远超实际需要的池上限；redis 一变慢就会变成
// 放大器：请求堆积 -> 建更多连接 -> redis 更慢。
//
// 三个超时同样重要：go-redis v6 的连接读写和排队都不接受 context，上游 ctx 到期之后
// 连接仍会等满 ReadTimeout 才归还，排队中的再叠加 PoolTimeout。按 Little's Law，连接
// 数需求等于请求速率乘以单请求耗时，压缩耗时比限制连接数更接近问题本源。
//
// 取值与 cachext 的 redis backend 保持一致。需要放宽的服务显式配置对应键覆盖，
// <NS>poolsize 配 0 则回到 go-redis 自己的默认值。
const (
	defaultPoolSize    = 100
	defaultReadTimeout = 200 * time.Millisecond
	defaultPoolTimeout = 100 * time.Millisecond
	defaultIdleTimeout = 2 * time.Minute
)

func (c *RedisExt) Init(app *gobay.Application) error {
	if c.NS == "" {
		return errors.New("lack of NS")
	}
	c.app = app
	config := gobay.GetConfigByPrefix(app.Config(), c.NS, true)
	// 先补齐下划线写法（<NS>pool_size），否则会被 mapstructure 静默忽略
	gobay.NormalizeUnderscoreKeys(config)
	config.SetDefault("poolsize", defaultPoolSize)
	config.SetDefault("readtimeout", defaultReadTimeout)
	config.SetDefault("pooltimeout", defaultPoolTimeout)
	config.SetDefault("idletimeout", defaultIdleTimeout)
	opt := redis.Options{}
	if err := config.Unmarshal(&opt); err != nil {
		return err
	}
	c.prefix = config.GetString("prefix")
	c.redisclient = redis.NewClient(&opt)
	if observability.GetApmEnable() {
		c.apmable = true
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
	if c.apmable {
		return c.apmredisclient.WithContext(ctx).RedisClient()
	}
	return c.redisclient.WithContext(ctx)
}

func (c *RedisExt) EvalLua(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	cmd := c.Client(ctx).Eval(script, keys, args...)
	return cmd.Result()
}
