package cachext

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/RichardKnop/machinery/v1/log"
	"github.com/prometheus/client_golang/prometheus"
)

// CachedConfig save the param and config for a cached func
type CachedConfig struct {
	cache        *CacheExt
	ttl          time.Duration
	version      int64
	funcName     string
	makeCacheKey makeCacheKeyFunc
	getResult    cachedFunc
}

type cacheOption func(config *CachedConfig) error
type cachedFunc func(context.Context, []string, []int64) (interface{}, error)
type makeCacheKeyFunc func(string, int64, []string, []int64) string

// this func is the default makeCacheKey, use SetMakeCacheKey to override it
func defaultMakeCacheKey(funcName string, version int64, strArgs []string, intArgs []int64) string {
	inputs := make([]string, len(strArgs)+len(intArgs)+2)
	inputs[0] = funcName
	inputs[1] = strconv.FormatInt(version, 10)
	offset := 2
	for i, arg := range strArgs {
		inputs[i+offset] = url.QueryEscape(arg)
	}
	offset = len(strArgs) + 2
	for i, arg := range intArgs {
		inputs[i+offset] = strconv.FormatInt(arg, 10)
	}
	return strings.Join(inputs, "&")
}

// MakeCacheKey return the cache key of a function cache
func (c *CachedConfig) MakeCacheKey(strArgs []string, intArgs []int64) string {
	return c.makeCacheKey(c.funcName, c.version, strArgs, intArgs)
}

// GetResult
func (c *CachedConfig) GetResult(ctx context.Context, out interface{}, strArgs []string, intArgs []int64) error {
	cacheKey := c.MakeCacheKey(strArgs, intArgs)
	data, err := c.cache.backend.Get(ctx, c.cache.transKey(cacheKey))
	if err != nil {
		return err
	}
	labels := prometheus.Labels{prefixName: c.cache.prefix, funcName: c.funcName}
	if c.cache.requestCounter != nil {
		// Increment request counter.
		c.cache.requestCounter.With(labels).Inc()
	}
	if data != nil {
		if c.cache.hitCounter != nil {
			// Increment hit counter.
			c.cache.hitCounter.With(labels).Inc()
		}
		return decode(data, out)
	}
	res, err := c.getResult(ctx, strArgs, intArgs)
	if err != nil {
		return err
	}

	// 把结果放入缓存
	if encodedBytes, err := encode(res); err != nil {
		return err
	} else {
		err = c.cache.backend.Set(ctx, c.cache.transKey(cacheKey), encodedBytes, c.ttl)
		if err != nil {
			return err
		}
		return decode(encodedBytes, out)
	}
}

// Cached return a ptr with two function: MakeCacheKey and GetResult
func (c *CacheExt) Cached(funcName string, f cachedFunc, options ...cacheOption) *CachedConfig {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := c.cachedFuncName[funcName]; ok {
		log.ERROR.Printf(fmt.Sprintf("Cached Func: `%s` already exists!", funcName))
	}
	c.cachedFuncName[funcName] = void{}
	cacheFuncConf := &CachedConfig{
		ttl:          24 * 2 * time.Hour,
		version:      1,
		cache:        c,
		getResult:    f,
		funcName:     funcName,
		makeCacheKey: defaultMakeCacheKey,
	}
	for _, option := range options {
		if err := option(cacheFuncConf); err != nil {
			panic(err)
		}
	}
	return cacheFuncConf
}

// WithTTL set ttl to the CachedConfig object, ttl must be a positive number
func WithTTL(ttl time.Duration) cacheOption {
	return func(config *CachedConfig) error {
		if ttl < 0 {
			return errors.New("ttl should be positive duration")
		}
		config.ttl = ttl
		return nil
	}
}

// WithVersion set version to the cacheFuncConfig object, if you want a function's all cache
// update immediately, change the version.
func WithVersion(version int64) cacheOption {
	return func(config *CachedConfig) error {
		config.version = version
		return nil
	}
}

// WithMakeCacheKey you can write your own makeCacheKey, use this func to change the default makeCacheKey.
// first param means funcName, the second param means version, next params mean real function input param.
func WithMakeCacheKey(f makeCacheKeyFunc) cacheOption {
	return func(config *CachedConfig) error {
		config.makeCacheKey = f
		return nil
	}
}
