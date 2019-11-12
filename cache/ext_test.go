package cache

import (
	"fmt"
	"github.com/shanbay/gobay"
	"testing"
)

func Example() {
	var key = "cache_key"
	cache := new(CacheExt)
	cache.Set(key, "hello", 10)
	val := cache.Get(key)
	fmt.Println(val)
	// Output: "hello"
	f := func(key string, name string) ([]string, error) {
		// write your code here
		res := make([]string, 2)
		res[0] = key
		res[1] = name
		return res, nil
	}
	cachedFunc := cache.CachedFunc(false, 10, 1, f)
	if keyName, err := cachedFunc("hello", "world"); err != nil {
		fmt.Println(keyName.([]string))
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
	exts := map[string]gobay.Extention{
		"cache": cache,
	}
	app, _ := gobay.CreateApp("../config.yaml", "testing", exts)
	if app == nil {
		t.Errorf("Cache Set Key failed")
	}
	if err := cache.Set("cache_key_1", 100, 10); err != nil {
		t.Errorf("Cache Set Key failed")
	}
	if val := cache.Get("cache_key_1"); val != 10 {
		t.Errorf("Cache Get failed")
	}
}
