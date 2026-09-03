# 接入 Redis

这是直接接入Redis的功能，如果只是对CRUD的model进行缓存，建议直接去隔壁[接入cache](ext_cache_cn.md)部分。

## config 配置

打开 config.yaml 文件，这几个配置

```yaml
  redis_addr: 'redis:6379'
  redis_password: ''
  redis_db: 0
```

⚠️ **地址键必须写 `<NS>addr`，不能写 `<NS>host`。** 本扩展把配置直接 `Unmarshal` 进 [`redis.Options`](https://pkg.go.dev/github.com/go-redis/redis#Options)，而该结构体只有 `Addr` 字段、没有 `Host`；mapstructure 找不到对应字段会静默跳过，`Addr` 保持为空，go-redis 随后 fallback 到 `localhost:6379` —— 不报错，连接目标却是错的。

> 注意与 `cachext` 的区别：**`cache_host` 是生效的**。cachext 的 redis backend 里有一段兼容代码，`Addr` 为空时会回落到读 `<NS>host`（历史配置键）。redisext 没有这段兼容，所以只认 `addr`。看到 `cache_host` 不要跟着改。

## 加载 redis 用的 extension

- `app/extensions.go`

```go
package app

import (
  schema "helloworld/gen/entschema"

  "git.17bdc.com/backend-lib/gordon/sensorext"
  "git.17bdc.com/shanbay/protos-go/xyz"
  elasticsearchv7 "github.com/elastic/go-elasticsearch/v7"
  "entgo.io/ent/dialect"

"github.com/shanbay/gobay"
  "github.com/shanbay/gobay/extensions/redisext"
)

// Extensions defined Extensions to be used by init app
func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    "redis":     &redisext.RedisExt{NS: "redis_"},
    // ...
  }
}

var (
  Redis          *redisext.RedisExt
  // ...
)

func InitExts(app *gobay.Application) {
  Redis = app.Get("redis").Object().(*redisext.RedisExt)
  // ...
}

```

## 使用 redis

```go
  // 获取 redis Client
  redisClient := app.Redis.Client(ctx)

  // 从 redis 里读取值
  res, err := redisClient.Get(cacheKey).Result()
  if err != redis.Nil {
    // log.Printf("redis error: %v", err)
    return nil, false
  }
  
  // 写入 redis
  redisClient.Set(cacheKey, string(userLabelJSON), 24*time.Hour)

  // 删除 redis 数据
  redisClient.Del(cacheKeys...).Result()
```

## 连接池与超时默认值

从 1.2.10 起，`redisext` 在 `Init` 时会注入以下默认值，业务项目无需配置：

| 配置键                                                 | 默认值  | go-redis 原默认值                            |
| ------------------------------------------------------ | ------- | -------------------------------------------- |
| `<NS>poolsize`                                         | `100`   | `10 × NumCPU`（v6）/ `10 × GOMAXPROCS`（v9） |
| `<NS>readtimeout`                                      | `200ms` | `3s`                                         |
| `<NS>pooltimeout`                                      | `100ms` | ReadTimeout + 1s                             |
| `<NS>idletimeout`（v6）<br>`<NS>connmaxidletime`（v9） | `2m`    | `5m`（v6）/ `30m`（v9）                      |

`NumCPU()` 读的是宿主机核数而非容器的 CPU limit，多核节点上的容器会拿到远超实际需要的池上限；这个上限会在 redis 变慢时变成放大器（请求堆积 → 建更多连接 → redis 更慢）。三项超时的作用是限制单个连接被占用的时长——按 Little's Law，连接数需求等于请求速率乘以单请求耗时，压缩耗时比限制连接数更早生效。

需要放宽的服务显式配置对应键即可覆盖，`<NS>poolsize: 0` 回到 go-redis 自己的默认值。

⚠️ 键名两种写法都支持（`redis_poolsize` 与 `redis_pool_size` 等价，并存时以前者为准）。时间参数必须带单位。

⚠️ 地址键用 `<NS>addr`，`<NS>host` 不生效，原因见本文开头的配置说明。
