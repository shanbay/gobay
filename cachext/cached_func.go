package cachext

import (
	"errors"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

const Nil = cacheNil("cache result is nil")

type cacheNil string

func (e cacheNil) Error() string { return string(e) }

// cachedFuncConfig save the param and config for a cached func
type cachedFuncConfig struct {
	cache        *CacheExt
	cacheNil     bool
	ttl          int64
	version      int64
	funcName     string
	makeCacheKey func(string, int64, []string, []int64) string
	getResult    func([]string, []int64) (interface{}, error)
}

type cacheOption func(config *cachedFuncConfig) error

// this func is the default makeCacheKey, use SetMakeCacheKey to override it
func makeCacheKey(funcName string, version int64, strArgs []string, intArgs []int64) string {
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
func (c *cachedFuncConfig) MakeCacheKey(strArgs []string, intArgs []int64) string {
	return c.makeCacheKey(c.funcName, c.version, strArgs, intArgs)
}

// GetResult
func (c *cachedFuncConfig) GetResult(out interface{}, strArgs []string, intArgs []int64) error {
	cacheKey := c.MakeCacheKey(strArgs, intArgs)
	data, err := c.cache.backend.Get(c.cache.transKey(cacheKey))
	if err != nil {
		return err
	}
	if data != nil {
		if c.cacheNil && decodeIsNil(data) {
			// 无法直接把out设置为nil，这里通过返回特殊的错误来表示nil。调用方需要判断
			return Nil
		}
		return decode(data, out)
	}
	res, err := c.getResult(strArgs, intArgs)
	if err != nil {
		return err
	}
	// 函数返回值与cacheNil需要设置的值相同，报错
	if c.cacheNil && decodeIsNil(res) {
		return errors.New("Your response is conflict with cacheNil value")
	}

	status := [2]bool{res == nil, c.cacheNil} // 函数返回值与是否cacheNil状态判断
	cacheNilHited := [2]bool{true, true}      // 函数返回值是nil，同时cacheNil。
	noNeedCacheNil := [2]bool{true, false}    // 函数返回值是nil，不cacheNil。

	switch status {
	case cacheNilHited:
		// Set nil 会保存一个[]byte{192}的结构到backend中
		if err = c.cache.Set(cacheKey, nil, c.ttl); err != nil {
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
			err = c.cache.backend.Set(c.cache.transKey(cacheKey), encodedBytes, ttl2duration(c.ttl))
			if err != nil {
				return err
			}
			return decode(encodedBytes, out)
		}
	}
}

// Cached return a ptr with two function: MakeCacheKey and GetResult
func (c *CacheExt) Cached(f func([]string, []int64) (interface{}, error), options ...cacheOption) (*cachedFuncConfig, error) {
	var defaultTTL int64 = 60 * 60 * 24 * 2
	cacheFuncConf := &cachedFuncConfig{
		ttl: defaultTTL, cacheNil: false, version: 1, getResult: f, cache: c,
		funcName:     runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(),
		makeCacheKey: makeCacheKey,
	}
	for _, option := range options {
		if err := option(cacheFuncConf); err != nil {
			return (*cachedFuncConfig)(nil), err
		}
	}
	return cacheFuncConf, nil
}

// SetTTL set ttl to the cachedFuncConfig object, ttl must be a positive number
func SetTTL(ttl int64) cacheOption {
	return func(config *cachedFuncConfig) error {
		if ttl <= 0 {
			return errors.New("`ttl` should be a positive number")
		}
		config.ttl = ttl
		return nil
	}
}

// SetVersion set version to the cacheFuncConfig object, if you want a function's all cache
// update immediately, change the version.
func SetVersion(version int64) cacheOption {
	return func(config *cachedFuncConfig) error {
		config.version = version
		return nil
	}
}

// SetCacheNil set whether cacheNil to cacheFuncConfig, if cacheNil seted and function returns nil, GetResult will return Nil
// cacheNil stored in redis with []byte{192} 0xC0
func SetCacheNil(cacheNil bool) cacheOption {
	return func(config *cachedFuncConfig) error {
		config.cacheNil = cacheNil
		return nil
	}
}

// SetMakeCacheKey you can write your own makeCacheKey, use this func to change the default makeCacheKey to yours.
func SetMakeCacheKey(f func(string, int64, []string, []int64) string) cacheOption {
	return func(config *cachedFuncConfig) error {
		config.makeCacheKey = f
		return nil
	}
}
