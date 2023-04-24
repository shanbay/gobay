# 接入 Redis

这是直接接入Redis的功能，如果只是对CRUD的model进行缓存，建议直接去隔壁[接入cache](ext_cache_cn.md)部分。

## config 配置

打开 config.yaml 文件，这几个配置

```yaml
  redis_host: 'redis:6379'
  redis_password: ''
  redis_db: 0
```

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
