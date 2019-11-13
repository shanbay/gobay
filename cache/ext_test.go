package cache

import (
	"fmt"
	"github.com/shanbay/gobay"
	"testing"
)

func Example() {
	cache := &CacheExt{}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	gobay.CreateApp("../testdata/", "testing", exts)

	var key = "cache_key"
	cache.Set(key, "hello", 10)
	res, _ := cache.Get(key)
	fmt.Println(res)
	// Output: hello
	f := func(key string, name string) ([]string, error) {
		// write your code here
		res := make([]string, 2)
		res[0] = key
		res[1] = name
		return res, nil
	}
	cachedFunc, _ := cache.CachedFunc(f, 10, 1)
	if _, err := cachedFunc("hello", "world"); err != nil {
		// Output: ["hello", "world"], nil
	}
	if cacheKey, err := cache.MakeCacheKey(f, 1, "hello", "world"); err != nil {
		exists := cache.Exists(cacheKey)
		fmt.Println(exists)
		// Output: true
	}
}

func TestCacheExt_Get_Set(t *testing.T) {
	cache := &CacheExt{}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	app, app_err := gobay.CreateApp("../testdata/", "testing", exts)
	if app == nil {
		t.Error("Create App failed", app_err)
	}
	if err := cache.Set("cache_key_1", "100", 10); err != nil {
		t.Errorf("Cache Set Key failed")
	}
	if val, err := cache.Get("cache_key_1"); err != nil || val.(string) != "100" {
		t.Log("err", val, err)
		t.Errorf("cache get set error")
	}
}

func TestCacheExt_CachedFunc(t *testing.T) {
	// 准备数据
	cache := &CacheExt{}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	app, app_err := gobay.CreateApp("../testdata/", "testing", exts)
	if app == nil {
		t.Error("Create App failed", app_err)
	}
	// 函数真实调用次数
	call_times := 0
	f := func(key1 string, key2 string, key3 int64) (string, error) {
		call_times += 1
		return key1 + "*" + key2, nil
	}
	cached_func, _ := cache.CachedFunc(f, 10, 1)
	if val, err := cached_func("hello", "world", int64(12)); err != nil || val.(string) != "hello*world" {
		t.Error(err)
		t.Error("result error")
	}
	// 这里都命中缓存，call_times不会增加
	cached_func("hello", "world", int64(12))
	cached_func("hello", "world", int64(12))
	cached_func("hello", "world", int64(12))
	if call_times != 1 {
		t.Error("Cached func not work", call_times)
	}
	// 这里删除缓存key
	if cache_key, err := cache.MakeCacheKey(f, 1, "hello", "world", int64(12)); err != nil {
		t.Error(err)
	} else {
		cache.Delete(cache_key)
	}
	// 再次调用，不命中缓存
	cached_func("hello", "world", int64(12))
	if call_times != 2 {
		t.Error("Cached func not work", call_times)
	}
	// 测试函数返回值为nil的情况
	call_times = 0
	ff := func(key1 string, key2 string, key3 int64) (interface{}, error) {
		call_times += 1
		return nil, nil
	}
	//  cached_ff, _ := cache.CachedFunc(ff, 10, 1)
	//  if val, err := cached_ff("hello", "world", int64(12)); err != nil || val != nil {
	//  	t.Error("cached none failed")
	//  	t.Error(err, val)
	//  }
	//  if call_times != 1 {
	//  	t.Error("cached none failed")
	//  }
	//  if val, err := cached_ff("hello", "world", int64(12)); val != nil || err != nil {
	//  	t.Log("cached none failed", val, err)
	//  }
	//  cached_ff("hello", "world", int64(12))
	//  cached_ff("hello", "world", int64(12))
	//  if call_times != 1 {
	//  	t.Error("cached none failed", call_times)
	//  }
	// 没有配置cache_none参数，每次都会调用
	not_cached_ff, _ := cache.CachedFunc(ff, 10, 1)
	call_times = 0
	val, err := not_cached_ff("hello", "world", int64(12))
	if val != nil || err != nil {
		t.Error("cache return wrong data")
	}
	not_cached_ff("hello", "world", int64(12))
	not_cached_ff("hello", "world", int64(12))
	if call_times != 3 {
		t.Error("cache nont error")
		t.Error(call_times)
	}
	type node struct {
		Name string
		Ids  []string
	}
	// 存取复杂结构
	type myData struct {
		Value1 int
		Value2 string
		Value3 []node
	}
	complex_ff := func() (node, error) {
		call_times += 1
		mydata := &myData{}
		mydata.Value1 = 100
		mydata.Value2 = "thre si a verty conplex data {}{}"
		mydata.Value3 = []node{node{"这是第一个node", []string{"id1", "id2", "id3"}}, node{"这是第二个node", []string{"id4", "id5", "id6"}}}
		return mydata.Value3[0], nil
	}
	cached_complex_ff, _ := cache.CachedFunc(complex_ff, 10, 1)
	call_times = 0
	cached_complex_ff()
	cached_complex_ff()
	cached_complex_ff()
	complex_val, err := cached_complex_ff()
	if call_times != 1 {
		t.Error("Error happened in cache complex", call_times)
	}
	if complex_val.(map[string]interface{})["Name"] != "这是第一个node" {
		t.Error("Data is wrong in cache complex")
	}
}
