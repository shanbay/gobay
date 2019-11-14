package cache

import (
	"fmt"
	"github.com/shanbay/gobay"
	"testing"
)

func ExampleGet() {
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
}
func ExampleCachedFunc() {
	cache := &CacheExt{}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	gobay.CreateApp("../testdata/", "testing", exts)

	var call_times = 0

	f := func(key string, name string) ([]string, error) {
		// write your code here
		call_times += 1
		res := make([]string, 2)
		res[0] = key
		res[1] = name
		return res, nil
	}
	cachedFunc, _ := cache.CachedFunc(f, 10, 1)
	cachedFunc("hello", "world")
	cachedFunc("hello", "world")
	res, err := cachedFunc("hello", "world")
	cachedFunc("hello", "world")
	fmt.Println(res, call_times, err)
	// Output: [hello world] 1 <nil>
}

func ExampleCachedFunc2() {
	cache := &CacheExt{}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	gobay.CreateApp("../testdata/", "testing", exts)
	var call_times = 0

	f := func(names ...string) ([]string, error) {
		// write your code here
		call_times += 1
		res := make([]string, 2)
		res[0] = names[0]
		res[1] = "world"
		return res, nil
	}
	cachedFunc, _ := cache.CachedFunc(f, 10, 1)
	args := make([]interface{}, 2)
	// 在使用...语法时，args 目前只能写成[]interface{}，后续再研究有没有办法写成[]string
	args[0] = "hello"
	args[1] = "mock"
	cachedFunc(args...)
	cachedFunc(args...)
	res, err := cachedFunc(args...)
	cachedFunc(args...)
	fmt.Println(res, call_times, err)
	// Output: [hello world] 1 <nil>
}

func TestCacheExt_Operation(t *testing.T) {
	cache := &CacheExt{}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	gobay.CreateApp("../testdata/", "testing", exts)
	// Get Set
	if err := cache.Set("cache_key_1", "100", 10); err != nil {
		t.Errorf("Cache Set Key Failed")
	}
	if val, err := cache.Get("cache_key_1"); err != nil || val.(string) != "100" {
		t.Log(val, err)
		t.Errorf("Cache Get Key Failed")
	}
	// SetStruct GetStruct
	type node struct {
		Name string
		Ids  []string
	}
	type myData struct {
		Value1 int
		Value2 string
		Value3 []node
	}
	mydata := myData{}
	mydata.Value1 = 100
	mydata.Value2 = "thre si a verty conplex data {}{}"
	mydata.Value3 = []node{node{"这是第一个node", []string{"id1", "id2", "id3"}}, node{"这是第二个node", []string{"id4", "id5", "id6"}}}
	if err := cache.SetStruct("cache_key_2", mydata, 10); err != nil {
		t.Log(err)
		t.Errorf("Cache SetStruct Failed")
	}
	val := &myData{}
	if exist, err := cache.GetStruct("cache_key_2", val); (*val).Value2 != mydata.Value2 || err != nil || exist == false {
		t.Log(exist, err, *val)
		t.Errorf("Cache GetStruct Failed")
	}
	// SetMany GetMany
	many_key := []string{"m1", "m2", "m3", "m4"}
	many_value := []interface{}{"hello", "world", "hello", "go"}
	if err := cache.SetMany(many_key, many_value, 10); err != nil {
		t.Log(err)
		t.Errorf("Cache SetMany Failed")
	}
	many_res := cache.GetMany(many_key)
	if many_res[0].(string) != "hello" {
		t.Log(many_value)
		t.Log(many_res)
		t.Errorf("Cache GetMany Failed")
	}
	// Delete Exists
	cache.Set("cache_key_3", "golang", 10)
	if res := cache.Exists("cache_key_3"); res != true {
		t.Log(res)
		t.Errorf("Cache Exists Failed")
	}
	if res := cache.Delete("cache_key_3"); res != 1 {
		t.Log(res)
		t.Errorf("Cache Delete Failed")
	}
	if res := cache.Exists("cache_key_3"); res != false {
		t.Log(res)
		t.Errorf("Cache Exists Failed")
	}
	if res := cache.Delete("cache_key_3"); res != 0 {
		t.Log(res)
		t.Errorf("Cache Delete Failed")
	}
	// Expire TTL
	cache.Set("cache_key_4", "hello", 10)
	if res := cache.TTL("cache_key_4"); res != 10 {
		t.Log(res)
		t.Errorf("Cache TTL Failed")
	}
	if res := cache.Expire("cache_key_4", 20); res != true {
		t.Log(res)
		t.Errorf("Cache Expire Failed")
	}
	if res := cache.TTL("cache_key_4"); res != 20 {
		t.Log(res)
		t.Errorf("Cache TTL Failed")
	}
}

func TestCacheExt_CachedFunc_Common(t *testing.T) {
	// 准备数据
	cache := &CacheExt{}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	gobay.CreateApp("../testdata/", "testing", exts)
	call_times := 0
	// common 主要测试返回值为：int []int string []string bool []bool
	// []string
	f := func(key1 string, key2 string, key3 int64) ([]string, error) {
		call_times += 1
		res := make([]string, 2)
		res[0] = key1
		res[1] = key2
		return res, nil
	}
	cached_func, _ := cache.CachedFunc(f, 10, 1)
	cached_func("hello", "world", int64(12))
	if val, err := cached_func("hello", "world", int64(12)); err != nil || val.([]string)[0] != "hello" {
		t.Log(err)
		t.Errorf("result error")
	}
	cached_func("hello", "world", int64(12))
	cached_func("hello", "world", int64(12))
	cached_func("hello", "world", int64(12))
	if call_times != 1 {
		t.Log(call_times)
		t.Errorf("CachedFunc Not Work For []string")
	}
	// make cache key
	if cache_key, err := cache.MakeCacheKey(f, 1, "hello", "world", int64(12)); err != nil {
		t.Error(err)
	} else {
		cache.Delete(cache_key)
	}
	cached_func("hello", "world", int64(12))
	if call_times != 2 {
		t.Errorf("MakeCacheKey Not Work")
	}
	// string
	f_str := func(name string) (string, error) {
		call_times += 1
		return name, nil
	}
	c_f_str, _ := cache.CachedFunc(f_str, 10, 1)
	call_times = 0
	c_f_str("hello")
	c_f_str("hello")
	c_f_str("hello")
	if val, err := c_f_str("hello"); val.(string) != "hello" || err != nil || call_times != 1 {
		t.Log(val, err, call_times)
		t.Errorf("CachedFunc Not Work For string")
	}
	// bool
	f_bool := func(na bool) (bool, error) { call_times += 1; return na, nil }
	c_f_bool, _ := cache.CachedFunc(f_bool, 10, 1)
	call_times = 0
	c_f_bool(true)
	c_f_bool(true)
	if val, err := c_f_bool(true); val.(bool) != true || err != nil || call_times != 1 {
		t.Log(val, err, call_times)
		t.Errorf("CachedFunc Not Work For bool")
	}
	// []bool
	f_bools := func(na ...bool) ([]bool, error) { call_times += 1; return []bool{true, false}, nil }
	c_f_bools, _ := cache.CachedFunc(f_bools, 10, 1)
	call_times = 0
	bools := make([]interface{}, 3)
	bools[0] = true
	bools[1] = true
	bools[2] = true
	c_f_bools(bools...)
	c_f_bools(bools...)
	if val, err := c_f_bools(bools...); val.([]bool)[0] != true || err != nil || call_times != 1 {
		t.Log(val, err, call_times)
		t.Errorf("CachedFunc Not Work For []bool")
	}
	// int
	f_int := func(name string) (int, error) { call_times += 1; return 1, nil }
	c_f_int, _ := cache.CachedFunc(f_int, 10, 1)
	call_times = 0
	c_f_int("well")
	c_f_int("well")
	if val, err := c_f_int("well"); val.(int) != 1 || err != nil || call_times != 1 {
		t.Log(val, err, call_times)
		t.Errorf("CachedFunc Not Work For int")
	}
	// []int
	f_ints := func(name string) ([]int, error) { call_times += 1; res := make([]int, 1); res[0] = 1; return res, nil }
	c_f_ints, _ := cache.CachedFunc(f_ints, 10, 1)
	call_times = 0
	c_f_ints("hello")
	c_f_ints("hello")
	c_f_ints("hello")
	if val, err := c_f_ints("hello"); val.([]int)[0] != 1 || err != nil || call_times != 1 {
		t.Log(val, err, call_times)
		t.Errorf("CachedFunc Not Work For []int")
	}
	// 测试函数返回值为nil的情况，golang只有interface{}类型的返回值才可以返回nil 不是很建议在cachefucn中这么写
	// 以下这种情况下返回的是空数组
	nil_func := func(name string) (interface{}, error) {
		call_times += 1
		if name == "nil" {
			return nil, nil
		}
		return name, nil
	}
	cached_nil_func, _ := cache.CachedFunc(nil_func, 10, 1)

	call_times = 0
	cached_nil_func("test")
	cached_nil_func("test")
	cached_nil_func("test")
	if cache_nil_res, err := cached_nil_func("test"); cache_nil_res.(string) != "test" || err != nil || call_times != 1 {
		t.Log(cache_nil_res, err, call_times)
		t.Errorf("cache nil func error happened")
	}
	call_times = 0
	cached_nil_func("nil")

	if cache_nil_res, err := cached_nil_func("nil"); cache_nil_res != nil || err != nil || call_times != 2 {
		t.Log(cache_nil_res, err, call_times)
		t.Errorf("cache nil func return nil succeed")
	}

}

func TestCacheExt_CachedFunc_Struct(t *testing.T) {
	// 准备数据
	cache := &CacheExt{}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	gobay.CreateApp("../testdata/", "testing", exts)
	call_times := 0
	// 函数返回值是struct
	type node struct {
		Name string
		Ids  []string
	}
	type myData struct {
		Value1 int
		Value2 string
		Value3 []node
	}
	complex_ff := func() (myData, error) {
		call_times += 1
		mydata := myData{}
		mydata.Value1 = 100
		mydata.Value2 = "thre si a verty conplex data {}{}"
		mydata.Value3 = []node{node{"这是第一个node", []string{"id1", "id2", "id3"}}, node{"这是第二个node", []string{"id4", "id5", "id6"}}}
		return mydata, nil
	}
	cached_complex_ff, _ := cache.CachedFunc(complex_ff, 10, 1)
	call_times = 0
	cached_complex_ff()
	cached_complex_ff()
	cached_complex_ff()
	complex_val, err := cached_complex_ff()
	if call_times != 1 || err != nil {
		t.Log(err, call_times)
		t.Errorf("Error happened in cache complex")
	}
	if complex_val.(myData).Value3[0].Name != "这是第一个node" {
		t.Errorf("Data is wrong in cache complex")
	}
}
