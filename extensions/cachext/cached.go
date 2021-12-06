package cachext

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/RichardKnop/machinery/v1/log"
	"github.com/prometheus/client_golang/prometheus"
)

const Nil = cacheNil("cache result is nil")

type cacheNil string

func (e cacheNil) Error() string { return string(e) }

// CachedConfig save the param and config for a cached func
type CachedConfig struct {
	cache        *CacheExt
	cacheNil     bool
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
		if c.cacheNil && decodeIsNil(data) {
			// 无法直接把out设置为nil，这里通过返回特殊的错误来表示nil。调用方需要判断
			return Nil
		}
		return decode(data, out)
	}
	res, err := c.getResult(ctx, strArgs, intArgs)
	if err != nil {
		return err
	}
	// 函数返回值与cacheNil需要设置的值相同，报错
	if c.cacheNil && decodeIsNil(res) {
		return errors.New("Your response is conflict with cacheNil value")
	}

	resIsNil := false
	// res 储存的是一个有类型的 nil 指针时，`== nil` 是 false
	if res == nil || (reflect.ValueOf(res).Kind() == reflect.Ptr && reflect.ValueOf(res).IsNil()) {
		resIsNil = true
	}
	status := [2]bool{resIsNil, c.cacheNil} // 函数返回值与是否cacheNil状态判断
	cacheNilHited := [2]bool{true, true}      // 函数返回值是nil，同时cacheNil。
	noNeedCacheNil := [2]bool{true, false}    // 函数返回值是nil，不cacheNil。

	switch status {
	case cacheNilHited:
		// Set nil 会保存一个[]byte{192}的结构到backend中
		nilBytes, _ := encode(nil)
		if err = c.cache.backend.Set(ctx, c.cache.transKey(cacheKey), nilBytes, c.ttl); err != nil {
			return err
		}
		return Nil
	case noNeedCacheNil:
		return Nil
	default:
		// 函数返回值非空，把结果放入缓存。不管是否cacheNil
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
		cacheNil:     false,
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

// WithCacheNil set whether cacheNil to cacheFuncConfig, if cacheNil seted and function returns nil, GetResult will return Nil
// cacheNil stored in redis with []byte{192} 0xC0
func WithCacheNil(cacheNil bool) cacheOption {
	return func(config *CachedConfig) error {
		config.cacheNil = cacheNil
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
