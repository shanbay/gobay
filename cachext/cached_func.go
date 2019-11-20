package cachext

import (
	"errors"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

var (
	Nil = errors.New("Your function's result is nil")
)

// cachedFuncConfig save the param and config for a cached func
type cachedFuncConfig struct {
	cache        *CacheExt
	ttl          int64
	cacheNil     bool
	version      int64
	getResult    func(arg1 []string, arg2 []int64) (interface{}, error)
	makeCacheKey func([]string, []int64) string
}

func (c *cachedFuncConfig) defaultMakeCacheKey(arg1 []string, arg2 []int64) string {
	inputs := make([]string, len(arg1)+len(arg2)+2)
	inputs[0] = runtime.FuncForPC(reflect.ValueOf(c.getResult).Pointer()).Name()
	inputs[1] = strconv.FormatInt(c.version, 10)
	offset := 2
	for i, arg := range arg1 {
		inputs[i+offset] = arg
	}
	offset = 2 + len(arg1)
	for i, arg := range arg2 {
		inputs[i+offset] = strconv.FormatInt(arg, 10)
	}
	for i, input := range inputs {
		inputs[i] = url.QueryEscape(input)
	}
	return strings.Join(inputs, "&")
}

// MakeCacheKey return the cache key of a function cache
func (c *cachedFuncConfig) MakeCacheKey(arg1 []string, arg2 []int64) string {
	return c.makeCacheKey(arg1, arg2)
}

// GetResult
func (c *cachedFuncConfig) GetResult(out interface{}, arg1 []string, arg2 []int64) error {
	cacheKey := c.MakeCacheKey(arg1, arg2)
	data, err := c.cache.backend.Get(c.cache.transKey(cacheKey))
	if err != nil {
		return err
	}
	if data != nil {
		if c.cacheNil && c.cache.decodeIsNil(data) {
			// 无法直接把out设置为nil，这里通过返回特殊的错误来表示nil。调用方需要判断
			return Nil
		}
		return c.cache.decode(data, out)
	}
	res, err := c.getResult(arg1, arg2)
	if err != nil {
		return err
	}
	// 函数返回值与cacheNil需要设置的值相同，报错
	if c.cacheNil && c.cache.decodeIsNil(res) {
		return errors.New("Your response is conflict with cacheNil value")
	}

	// 把函数返回结果写回缓存
	switch [2]bool{res == nil, c.cacheNil} {
	case [2]bool{true, true}: // 函数返回值是nil，同时cacheNil。Set nil 会保存一个[]byte{192}的结构到backend中
		if err = c.cache.Set(cacheKey, nil, c.ttl); err != nil {
			return err
		}
	case [2]bool{true, false}: // 函数返回值是nil，不cacheNil。
	case [2]bool{false, true}, [2]bool{false, false}: // 函数返回值非空，把结果放入缓存。不管是否cacheNil
		if err = c.cache.Set(cacheKey, res, c.ttl); err != nil {
			return err
		}
	}

	if res == nil {
		// 无法直接把out设置为nil，这里通过返回特殊的错误来表示nil。调用方需要判断
		return Nil
	} else {
		// 这里先encode再decode是为了把res的值写到out中
		encoded_bytes, _ := c.cache.encode(res)
		c.cache.decode(encoded_bytes, out)
		return nil
	}
}

type cacheOption func(config *cachedFuncConfig) error

// Cached return a ptr with two function: MakeCacheKey and GetResult
func (c *CacheExt) Cached(f func(arg1 []string, arg2 []int64) (interface{}, error), options ...cacheOption) (*cachedFuncConfig, error) {
	var defaultTTL int64 = 60 * 60 * 24 * 2
	cacheFuncConf := &cachedFuncConfig{ttl: defaultTTL, cacheNil: false, version: 1, getResult: f, cache: c}
	cacheFuncConf.makeCacheKey = cacheFuncConf.defaultMakeCacheKey
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
func SetMakeCacheKey(f func([]string, []int64) string) cacheOption {
	return func(config *cachedFuncConfig) error {
		config.makeCacheKey = f
		return nil
	}
}
