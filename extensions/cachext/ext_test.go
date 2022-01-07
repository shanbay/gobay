package cachext_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"testing"
	"time"

	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/cachext"
	_ "github.com/shanbay/gobay/extensions/cachext/backend/memory"
	"github.com/stretchr/testify/assert"
)

func ExampleCacheExt_Set() {
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	var key = "cache_key"
	err := cache.Set(context.Background(), key, "hello", 10*time.Second)
	fmt.Println(err)
	var res string
	exists, err := cache.Get(context.Background(), key, &res)
	fmt.Println(exists, res, err)
	// Output:
	// <nil>
	// true hello <nil>
}

func ExampleCacheExt_CheckHealth() {
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	err := cache.CheckHealth(context.Background())
	fmt.Println(err)
	// Output:
	// <nil>
}

func ExampleCacheExt_Cached() {
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	var call_times = 0
	var err error

	f := func(_ context.Context, keys []string, arg []int64) (interface{}, error) {
		call_times += 1
		res := make([]string, 2)
		res[0] = keys[0]
		res[1] = keys[0]
		return res, nil
	}
	cachedFunc := cache.Cached("f", f, cachext.WithTTL(10*time.Second), cachext.WithVersion(1), cachext.WithMakeCacheKey(
		func(funcName string, version int64, strArgs []string, intArgs []int64) string {
			return strings.Join(strArgs, "_")
		},
	))

	zero_res := make([]string, 2)
	for i := 0; i <= 1; i++ {
		if err := cachedFunc.GetResult(context.Background(), &zero_res, []string{"hello", "world"}, []int64{}); err != nil {
			fmt.Println("Cache set Failed")
		}
	}
	err = cachedFunc.GetResult(context.Background(), &zero_res, []string{"hello", "world"}, []int64{})
	fmt.Println(zero_res, call_times, err)
	// Output: [hello hello] 1 <nil>
}

func ExampleCacheExt_SetMany() {
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}
	// SetMany GetMany
	many_map := make(map[string]interface{})
	many_map["1"] = "hello"
	many_map["2"] = []bool{true, true}
	err := cache.SetMany(context.Background(), many_map, 10*time.Second)
	fmt.Println(err)

	many_res := make(map[string]interface{})
	// 填上零值
	var str1 string
	val2 := []bool{}
	many_res["1"] = &str1
	many_res["2"] = &val2
	err = cache.GetMany(context.Background(), many_res)
	fmt.Println(err, *(many_res["1"].(*string)), *(many_res["2"].(*[]bool)))
	// Output: <nil>
	// <nil> hello [true true]
}

func TestCacheExt_Operation(t *testing.T) {
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	// Get Set
	if err := cache.Set(context.Background(), "cache_key_1", "100", 10*time.Second); err != nil {
		t.Errorf("Cache Set Key Failed")
	}
	var cache_val string
	if exists, err := cache.Get(context.Background(), "cache_key_1", &cache_val); exists == false || err != nil || cache_val != "100" {
		t.Log(exists, cache_val, err)
		t.Errorf("Cache Get Key Failed")
	}
	// Set Get
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
	mydata.Value3 = []node{{"这是第一个node", []string{"id1", "id2", "id3"}}, {"这是第二个node", []string{"id4", "id5", "id6"}}}
	if err := cache.Set(context.Background(), "cache_key_2", mydata, 10*time.Second); err != nil {
		t.Log(err)
		t.Errorf("Cache Set Failed")
	}
	val := &myData{}
	if exist, err := cache.Get(context.Background(), "cache_key_2", val); (*val).Value2 != mydata.Value2 || err != nil || exist == false {
		t.Log(exist, err, *val)
		t.Errorf("Cache Get Failed")
	}
	// SetMany GetMany
	many_map := make(map[string]interface{})
	many_map["m1"] = "hello"
	many_map["m2"] = "100"
	many_map["m3"] = "true"
	if err := cache.SetMany(context.Background(), many_map, 10*time.Second); err != nil {
		t.Log(err)
		t.Errorf("Cache SetMany Failed")
	}

	many_res := make(map[string]interface{})
	// 填上零值
	var str1, str2, str3 string
	many_res["m1"] = &str1
	many_res["m2"] = &str2
	many_res["m3"] = &str3
	if err := cache.GetMany(context.Background(), many_res); err != nil ||
		*(many_res["m1"].(*string)) != "hello" ||
		*(many_res["m2"].(*string)) != "100" ||
		*(many_res["m3"].(*string)) != "true" {
		t.Log(err, "many_res value:", many_res)
		t.Errorf("Cache GetMany Failed")
	}
	// Delete Exists
	if err := cache.Set(context.Background(), "cache_key_3", "golang", 10*time.Second); err != nil {
		t.Errorf("Cache set Failed")
	}
	if err := cache.Set(context.Background(), "cache_key_4", "gobay", 10*time.Second); err != nil {
		t.Errorf("Cache set Failed")
	}
	if res := cache.Exists(context.Background(), "cache_key_3"); res != true {
		t.Log(res)
		t.Errorf("Cache Exists Failed")
	}
	if res := cache.Delete(context.Background(), "cache_key_3"); res != true {
		t.Log(res)
		t.Errorf("Cache Delete Failed")
	}
	if res := cache.Exists(context.Background(), "cache_key_3"); res != false {
		t.Log(res)
		t.Errorf("Cache Exists Failed")
	}
	if res := cache.Delete(context.Background(), "cache_key_3"); res != false {
		t.Log(res)
		t.Errorf("Cache Delete Failed")
	}
	// DeleteMany
	keys := []string{"cache_key_3", "cache_key_4"}
	if res := cache.Exists(context.Background(), "cache_key_4"); res != true {
		t.Log(res)
		t.Errorf("Cache Exists Failed")
	}
	if res := cache.DeleteMany(context.Background(), keys...); res != true {
		t.Log(res)
		t.Errorf("Cache DeleteMany Failed")
	}
	if res := cache.Exists(context.Background(), "cache_key_4"); res != false {
		t.Log(res)
		t.Errorf("Cache Exists Failed")
	}
	if res := cache.DeleteMany(context.Background(), keys...); res != false {
		t.Log(res)
		t.Errorf("Cache DeleteMany Failed")
	}
	// Expire TTL
	if err := cache.Set(context.Background(), "cache_key_4", "hello", 10*time.Second); err != nil {
		t.Errorf("Cache set Failed")
	}
	if res := cache.TTL(context.Background(), "cache_key_4"); res < 9*time.Second || res > 10*time.Second {
		t.Log(res)
		t.Errorf("Cache TTL Failed")
	}
	if res := cache.Expire(context.Background(), "cache_key_4", 20*time.Second); res != true {
		t.Log(res)
		t.Errorf("Cache Expire Failed")
	}
	if res := cache.TTL(context.Background(), "cache_key_4"); res < 19*time.Second || res > 20*time.Second {
		t.Log(res)
		t.Errorf("Cache TTL Failed")
	}
}

func TestCacheExt_Cached_Common(t *testing.T) {
	// 准备数据
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

	call_times := 0
	// common 主要测试返回值为： []string string bool []bool int []int nil
	// []string
	f_strs := func(_ context.Context, keys []string, args []int64) (interface{}, error) {
		call_times += 1
		res := make([]string, 2)
		res[0] = keys[0]
		res[1] = keys[0]
		return res, nil
	}
	c_f_strs := cache.Cached("f_strs", f_strs, cachext.WithTTL(10*time.Second))
	cache_key := c_f_strs.MakeCacheKey([]string{"hello", "world"}, []int64{12})
	cache.Delete(context.Background(), cache_key)
	call_times = 0
	str_list := make([]string, 2)

	for i := 0; i <= 2; i++ {
		err := c_f_strs.GetResult(context.Background(), &str_list, []string{"hello", "world"}, []int64{12})
		if err != nil {
			t.Errorf("GetResult failed")
		}
	}
	str_list = make([]string, 2)
	if err := c_f_strs.GetResult(context.Background(), &str_list, []string{"hello", "world"}, []int64{12}); err != nil || str_list[0] != "hello" || str_list[1] != "hello" || call_times != 1 {
		t.Log(str_list, err, call_times)
		t.Errorf("Cache str_list failed")
	}
	// make cache key
	cache.Delete(context.Background(), cache_key)
	if err := c_f_strs.GetResult(context.Background(), &str_list, []string{"hello", "world"}, []int64{12}); call_times != 2 {
		t.Log(str_list, err, call_times)
		t.Errorf("Cache str_list failed")
	}
	// string
	f_str := func(_ context.Context, keys []string, args []int64) (interface{}, error) {
		call_times += 1
		return keys[0], nil
	}
	c_f_str := cache.Cached("f_str", f_str, cachext.WithTTL(10*time.Second))
	call_times = 0
	str := ""

	for i := 0; i <= 2; i++ {
		err := c_f_str.GetResult(context.Background(), &str, []string{"hello"}, []int64{})
		if err != nil {
			t.Errorf("GetResult failed")
		}
	}
	if err := c_f_str.GetResult(context.Background(), &str, []string{"hello"}, []int64{}); str != "hello" || err != nil || call_times != 1 {
		t.Log(str, err, call_times)
		t.Errorf("Cached str failed")
	}
	// bool
	f_bool := func(_ context.Context, keys []string, args []int64) (interface{}, error) {
		call_times += 1
		return true, nil
	}
	c_f_bool := cache.Cached("f_bool", f_bool, cachext.WithTTL(10*time.Second))
	call_times = 0
	res_bool := false

	for i := 0; i <= 1; i++ {
		err := c_f_bool.GetResult(context.Background(), &res_bool, []string{"hello", "world"}, []int64{})
		if err != nil {
			t.Errorf("GetResult failed")
		}
	}
	res_bool = false
	if err := c_f_bool.GetResult(context.Background(), &res_bool, []string{"hello", "world"}, []int64{}); !res_bool || err != nil || call_times != 1 {
		t.Log(res_bool, err, call_times)
		t.Errorf("Cached bool failed")
	}
	// []bool
	f_bools := func(_ context.Context, keys []string, args []int64) (interface{}, error) {
		call_times += 1
		return []bool{true, false, true}, nil
	}
	c_f_bools := cache.Cached("f_bools", f_bools, cachext.WithTTL(10*time.Second))
	call_times = 0
	bools := make([]bool, 3)
	bools[0] = false
	bools[1] = false
	bools[2] = false
	for i := 0; i <= 1; i++ {
		err := c_f_bools.GetResult(context.Background(), &bools, []string{}, []int64{})
		if err != nil {
			t.Errorf("GetResult failed")
		}
	}
	bools[0] = false
	bools[1] = false
	bools[2] = false
	if err := c_f_bools.GetResult(context.Background(), &bools, []string{}, []int64{}); bools[0] != true || err != nil || call_times != 1 {
		t.Log(bools, err, call_times)
		t.Errorf("Cached []bool failed")
	}
	// int
	f_int := func(_ context.Context, names []string, args []int64) (interface{}, error) {
		call_times += 1
		return 1, nil
	}
	c_f_int := cache.Cached("f_int", f_int, cachext.WithTTL(10*time.Second))
	call_times = 0
	var int_res int
	for i := 0; i <= 1; i++ {
		err := c_f_int.GetResult(context.Background(), &int_res, []string{"well"}, []int64{})
		if err != nil {
			t.Errorf("GetResult failed")
		}
	}
	if err := c_f_int.GetResult(context.Background(), &int_res, []string{"well"}, []int64{}); int_res != 1 || err != nil || call_times != 1 {
		t.Log(int_res, err, call_times)
		t.Errorf("Cached int failed")
	}
	// []int
	f_ints := func(_ context.Context, names []string, arg []int64) (interface{}, error) {
		call_times += 1
		res := make([]int, 1)
		res[0] = 1
		return res, nil
	}
	c_f_ints := cache.Cached("f_ints", f_ints, cachext.WithTTL(10*time.Second))
	call_times = 0
	ints_res := make([]int, 1)
	for i := 0; i <= 2; i++ {
		err := c_f_ints.GetResult(context.Background(), &ints_res, []string{"hello"}, []int64{})
		if err != nil {
			t.Errorf("GetResult failed")
		}
	}
	if err := c_f_ints.GetResult(context.Background(), &ints_res, []string{"hello"}, []int64{}); ints_res[0] != 1 || err != nil || call_times != 1 {
		t.Log(ints_res, err, call_times)
		t.Errorf("Cached []int failed")
	}
	// nil
	f_nil := func(_ context.Context, names []string, arg []int64) (interface{}, error) {
		call_times += 1
		return func() (*string, error) {
			return nil, nil
		}()
	}
	c_f_nil := cache.Cached("f_nil", f_nil, cachext.WithVersion(2), cachext.WithTTL(10*time.Second))
	nil_res := "content"
	call_times = 0
	for i := 0; i <= 3; i++ {
		err := c_f_nil.GetResult(context.Background(), &nil_res, []string{}, []int64{})
		if err != nil || nil_res != "" {
			t.Errorf(err.Error())
		}
	}
	if call_times != 4 {
		t.Log(nil_res, call_times)
	}
	nil_res = "content"
	cn_f_nil := cache.Cached("cn_f_nil", f_nil, cachext.WithVersion(5), cachext.WithTTL(10*time.Second))
	call_times = 0
	for i := 0; i <= 3; i++ {
		err := cn_f_nil.GetResult(context.Background(), &nil_res, []string{}, []int64{})
		if err != nil || nil_res != "" {
			t.Errorf(err.Error())
		}
	}
	if call_times != 1 {
		t.Log(nil_res, call_times)
	}
}

func TestCacheExt_Cached_Struct(t *testing.T) {
	// 准备数据
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}

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
		Value4 string
	}
	complex_ff := func(_ context.Context, strArgs []string, intArgs []int64) (interface{}, error) {
		call_times += 1
		mydata := myData{}
		mydata.Value1 = 100
		mydata.Value2 = "thre si a verty conplex data {}{}"
		some_str := "some str"
		mydata.Value4 = some_str
		mydata.Value3 = []node{{"这是第一个node", []string{"id1", "id2", "id3"}}, {"这是第二个node", []string{"id4", "id5", "id6"}}}
		return mydata, nil
	}
	cached_complex_ff := cache.Cached("complex_ff", complex_ff, cachext.WithTTL(10*time.Second))
	call_times = 0
	data := myData{}
	for i := 0; i <= 2; i++ {
		err := cached_complex_ff.GetResult(context.Background(), &data, []string{"hell"}, []int64{})
		if err != nil {
			t.Errorf("GetResult failed")
		}
	}
	data = myData{}
	err := cached_complex_ff.GetResult(context.Background(), &data, []string{"hell"}, []int64{})
	if call_times != 1 || err != nil || data.Value4 != "some str" {
		t.Log(data, err, call_times)
		t.Errorf("Cached struct failed")
	}
}

func Benchmark_SetMany(b *testing.B) {
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}
	// SetMany GetMany
	many_map := make(map[string]interface{})
	many_map["1"] = []string{"hello", "world", "golang", "cache"}
	many_map["2"] = []int{100, 200, 300, 400, 500}
	many_map["3"] = true
	many_map["4"] = make(map[string]int)
	many_map["4"].(map[string]int)["1"] = 200
	many_map["4"].(map[string]int)["2"] = 900
	many_map["4"].(map[string]int)["3"] = 1200
	for i := 0; i < b.N; i++ {
		err := cache.SetMany(context.Background(), many_map, 10*time.Second)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func Benchmark_GetMany(b *testing.B) {
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}
	many_map := make(map[string]interface{})
	many_map["1"] = []string{"hello", "world", "golang", "cache"}
	many_map["2"] = []int{100, 200, 300, 400, 500}
	many_map["3"] = true
	many_map["4"] = "wewe"
	many_map["5"] = 100
	many_map["6"] = make(map[string]int)
	many_map["6"].(map[string]int)["1"] = 200
	many_map["6"].(map[string]int)["2"] = 900
	many_map["6"].(map[string]int)["3"] = 1200
	if err := cache.SetMany(context.Background(), many_map, 10*time.Second); err != nil {
		fmt.Println(err)
	}
	for i := 0; i < b.N; i++ {
		many_res := make(map[string]interface{})
		// 填上零值
		val1 := []string{}
		many_res["1"] = &val1
		val2 := []int{}
		many_res["2"] = &val2
		val3 := false
		many_res["3"] = &val3
		val4 := ""
		many_res["4"] = &val4
		val5 := 0
		many_res["5"] = &val5
		val6 := make(map[string]interface{})
		many_res["6"] = &val6
		if err := cache.GetMany(context.Background(), many_res); err != nil {
			fmt.Println(err)
		}
	}
}

func Benchmark_Cached(b *testing.B) {
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	if _, err := gobay.CreateApp("../../testdata/", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}
	f := func(_ context.Context, name []string, args []int64) (interface{}, error) {
		many_map := make(map[string]string)
		many_map["1"] = "hello"
		many_map["2"] = "wewe"
		many_map["3"] = "true"
		many_map["4"] = "100"
		many_map["5"] = "wewe"
		return many_map, nil
	}
	cached_f := cache.Cached("Benchmark_Cached", f, cachext.WithTTL(10*time.Second))
	for i := 0; i < b.N; i++ {
		zero_map := make(map[string]string)
		if err := cached_f.GetResult(context.Background(), &zero_map, []string{"hello"}, []int64{}); err != nil ||
			zero_map["1"] != "hello" ||
			zero_map["2"] != "wewe" ||
			zero_map["3"] != "true" ||
			zero_map["4"] != "100" ||
			zero_map["5"] != "wewe" {
			fmt.Println(err)
		}
	}
}

func TestCacheExt_Cached_Monitor(t *testing.T) {
	// 准备数据
	cache := &cachext.CacheExt{NS: "cache_"}
	exts := map[gobay.Key]gobay.Extension{
		"cache": cache,
	}
	_, err := gobay.CreateApp("../../testdata/", "cachemonitored", exts)
	assert.Nil(t, err)

	fetchMetricData := func() string {
		resp, err := http.Get("http://localhost:2112/metrics")
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()
		rawData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		return string(rawData)
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Fatalf("error when start prometheus server: %v\n", err)
		}
	}()

	// Cache method
	f_str := func(_ context.Context, keys []string, args []int64) (interface{}, error) {
		return keys[0], nil
	}
	c_f_str := cache.Cached("f_str", f_str, cachext.WithTTL(10*time.Second))
	str := ""

	// Get result from function
	c_f_str.GetResult(context.Background(), &str, []string{"hello"}, []int64{})
	data := fetchMetricData()
	assert.Contains(t, data, `cache_request_counter{func_name="f_str",prefix_name="github"} 1`)
	assert.NotContains(t, data, `cache_hit_counter`)

	// Get result from cache
	c_f_str.GetResult(context.Background(), &str, []string{"hello"}, []int64{})
	data = fetchMetricData()
	assert.Contains(t, data, `cache_request_counter{func_name="f_str",prefix_name="github"} 2`)
	assert.Contains(t, data, `cache_hit_counter{func_name="f_str",prefix_name="github"} 1`)
}
