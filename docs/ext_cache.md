# 接入缓存

注意，这里的缓存只是用于缓存function结果单一用途的。如果有自定义的存入redis的需求，请参考隔壁左传[redis](docs/ext_redis.md)部分。

## config 配置

首先，配置config.yaml

```yaml
  cache_backend: 'redis'
  cache_prefix: 'helloworld'
  cache_host: 'redis:6379'
  cache_password: ''
  cache_db: 0
```

这里，我们使用redis作为缓存的储存工具。也可以在config里改为 `cache_backend: 'memory'` ，则会使用内存来储存缓存。

*redis可以让多个服务器之间共享缓存，memory则只能修改查看自己服务器的缓存，但性能会更加优秀。

## 设置加载时用的 extension

- `app/extensions.go`

```go
package app

import (
  "github.com/shanbay/gobay/extensions/cachext"
  _ "github.com/shanbay/gobay/extensions/cachext/backend/redis"
)

func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    // ...
    "cache":  &cachext.CacheExt{NS: "cache_"},
    // ...
  }
}

var (
  // ...
  Cache                      *cachext.CacheExt
  // ...
)

func InitExts(app *gobay.Application) {
  // ...
  Cache = app.Get("cache").Object().(*cachext.CacheExt)
  // ...
}
```

## 准备 model

cache 缓存需要包裹一定的 model 才好使用。这儿我们假设是隔壁[数据库](docs/ext_database.md)的 `models.SampleGetLastByName(ctx, name)`。

## 写包裹 model 的缓存代码

在 `app/models` 文件夹下创建 `app/models/cache.go` 文件，我们现在通常把所有model的缓存用的代码都放这个同一个文件里，方便统一管理。（如果model很多，也可以给每个model配置一个cache.go文件，或者直接写在对应的model里）

```go
package models

import (
  "context"
  "helloworld/app"
  schema "helloworld/gen/entschema"
  "time"

  "github.com/shanbay/gobay/extensions/cachext"
)

var (
  cachedSampleGetLastByName *cachext.CachedConfig
)

func InitCaches() {
  cache := app.Cache

  cachedSampleGetLastByName = cache.Cached(
    "SampleGetLastByName",
    func(ctx context.Context, strArgs []string, intArgs []int64) (interface{}, error) {
      return SampleGetLastByName(ctx, strArgs[0])
    },
    cachext.WithTTL(24*time.Hour),
  )
}

func CachedSampleGetLastByName(ctx context.Context, name string) (*schema.Sample, error) {
  // 注意：result必须是一个pointer，基础的string|bool|int不可以作为result直接使用，需要先创建一个type struct才行。
  result := &schema.Sample{}
  if err := cachedSampleGetLastByName.GetResult(
    ctx, result, []string{name}, []int64{},
  ); err != nil {
    return nil, err
  }
  return result, nil
}

func ClearCachedSampleGetLastByName(ctx context.Context, name string) {
  cacheKey := cachedSampleGetLastByName.MakeCacheKey([]string{name}, []int64{})
  app.Cache.DeleteMany(ctx, cacheKey)
}
```

## 调用缓存的方法

这样调用就行了，跟直接调用 db model 的调用方法类似，效果一样。

```go
result, err := models.CachedSampleGetLastByName(ctx, name)
```
