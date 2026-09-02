# 接入缓存

注意，这里的缓存只是用于缓存function结果单一用途的。如果有自定义的存入redis的需求，请参考隔壁左传[redis](ext_redis_cn.md)部分。

## config 配置

首先，配置config.yaml

```yaml
cache_backend: "redis"
cache_prefix: "helloworld"
cache_host: "redis:6379"
cache_password: ""
cache_db: 0
```

这里，我们使用redis作为缓存的储存工具。也可以在config里改为 `cache_backend: 'memory'` ，则会使用内存来储存缓存。

\*redis可以让多个服务器之间共享缓存，memory则只能修改查看自己服务器的缓存，但性能会更加优秀。

### 连接池等 redis 参数

从 v1.2.9 起，`cache_` 段支持 [`redis.Options`](https://pkg.go.dev/github.com/go-redis/redis#Options) 的全部字段，键名是「字段名去掉大小写」，前面加 NS 前缀：

```yaml
cache_poolsize: 20 # PoolSize
cache_pooltimeout: 500ms # PoolTimeout
cache_minidleconns: 2 # MinIdleConns
cache_readtimeout: 3s # ReadTimeout
```

**两个容易静默踩错的地方：**

- **键名不能带下划线。** `cache_poolsize` 生效，`cache_pool_size` 会被静默忽略，既不生效也不报错。配置解析走的是 mapstructure 的大小写不敏感匹配，它不会把下划线归一化掉。
- **时间类参数必须带单位。** `500ms` / `3s` 正确；写成 `500` 会被解析成 500 **纳秒**。

`cache_host` 是历史键名，等价于 `cache_addr`，两者都支持（同时配置时 `cache_addr` 优先）。两个都没配会直接启动失败，而不是静默连到 `localhost:6379`。

### PoolSize 的默认值

**gobay 从 v1.2.9 起把 `PoolSize` 默认为 20**，而不是 go-redis 自己的 `10 × NumCPU`。

原因是 `NumCPU()` 读的是宿主机核数而不是容器的 CPU limit，在多核节点上跑的容器会拿到一个远超实际需要的池上限。这个上限平时看不出来，一旦 redis 变慢就会变成放大器：请求变慢 → 连接被占住 → 池继续扩容 → redis 压力更大。设一个保守的上限，等于把压力挡在服务侧而不是转身建更多连接去压垮 redis。

**v9 backend 会自动让路。** 它用的是 `10 × GOMAXPROCS`，而 Go 1.25 起 GOMAXPROCS 默认会取 `min(可用 CPU 数, cgroup 的 quota/period)`。所以 v9 backend 在启动时会比较 `GOMAXPROCS` 和 `NumCPU`：

- **两者不等** → 说明确实有机制（Go 1.25 的 container-aware GOMAXPROCS，或 automaxprocs）已经把它调下来了，此时 go-redis 自己算的 `10 × GOMAXPROCS` 会随容器规格缩放，比固定值更合理，gobay **不再覆盖**它
- **两者相等** → 说明没生效，用 gobay 的默认值 20 兜底

判断的是运行结果而不是 `runtime.Version()`：Go 1.25 的这个行为由 `containermaxprocs` GODEBUG 控制，而它的默认值取决于**主模块（也就是你的项目）**go.mod 里的 `go` 指令——gobay 作为依赖库读不到那个值。工具链升到 1.25 但项目 go.mod 还写着 1.24 时，GOMAXPROCS 依然是宿主机核数，这时候放手就等于没修。

启动日志会说明走了哪条路径。

**v6 backend 不会**，它用的是 `runtime.NumCPU()`——那个值在进程启动时由 OS 固定，不受 GOMAXPROCS 影响，Go 1.25 和 automaxprocs 都改变不了它。v6 只能靠这里的固定默认值。

需要更大的池就显式配置 `cache_poolsize: <n>`。**要退回 go-redis 的原默认值，配 `cache_poolsize: 0`**，不需要回滚 gobay 版本。没有显式配置时，启动日志里会打印一行说明实际生效的值和来源。

`PoolTimeout` 保持 go-redis 的默认值（`ReadTimeout + 1s` = 4s），gobay 不替业务决定。这个值决定的是「等不到连接时多久放弃」，属于失败语义而不是资源上限——4 秒往往比上游的超时还长，等于把请求堆积留在自己进程里，建议按自己的 SLO 显式配置成 `300ms`–`500ms`。

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

cache 缓存需要包裹一定的 model 才好使用。这儿我们假设是隔壁[数据库](ext_database_cn.md)的 `models.SampleGetLastByName(ctx, name)`。

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
