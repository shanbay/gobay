# 接入限流 Ratelimit

对于一些 API ，你可能希望它对于用户有限制。常用的情况，比如对于一些不需要登陆就可以访问的 API ，不加 ratelimit 的话，可能会有被暴力访问的风险。

\*我们有网关层的 ratelimit，但是那个会设定的比较宽松，对于一些 API 可能不适用。

## 算法

我们的 ratelimit 使用了 github.com/go-redis/redis_rate/v9 这个库。目前官方没有注明算法，按照测试结果，应该是[令牌桶算法 Token Bucket](https://en.wikipedia.org/wiki/Token_bucket)。

基本就是，假设设定了 1 分钟 60 次的限制:

- 开始时，桶里有 60 个令牌，用户可以立刻使用掉当前的 60 个令牌。
- 令牌用完后，新令牌会以 每分钟 60 个 -> 每秒钟 1 个 的速度放入桶中。
- 每个请求使用一个令牌。

## config 配置

首先，配置 config.yaml

```yaml
ratelimit_backend: "redis"
ratelimit_prefix: "proj_rl"
ratelimit_host: "redis:6379"
ratelimit_password: ""
ratelimit_db: 4
```

这里，我们使用 redis 作为 ratelimit 同步用的储存工具。

## 设置加载时用的 extension

- `app/extensions.go`

```go
package app

import (
  "github.com/shanbay/gobay/extensions/ratelimitext"
  _ "github.com/shanbay/gobay/extensions/ratelimit/backend/redis"
)

func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    // ...
    "ratelimit":  &ratelimitext.RatelimitExt{NS: "ratelimit_"},
    // ...
  }
}

var (
  // ...
  Ratelimit                      *ratelimitext.RatelimitExt
  // ...
)

func InitExts(app *gobay.Application) {
  // ...
  Ratelimit = app.Get("ratelimit").Object().(*ratelimitext.RatelimitExt)
  // ...
}
```

## 给 API 设定 ratelimit

```go
func (s *helloworldProjectServer) healthCheckHealthHandler() health.HealthCheckHandler {
  return health.HealthCheckHandlerFunc(func(params health.HealthCheckParams) middleware.Responder {
    ctx := params.HTTPRequest.Context()

    // 设定每分钟最多10个请求：
    // 10代表10个请求，60代表每分钟/每60秒。
    // path 和 ip 会作为ratelimit用的key传入。
    ip := params.HTTPRequest.Header.Get("X-Forwarded-For")
    allow, err := app.Ratelimit.Allow(ctx, "healthCheckHealthHandler", ip, 10, 60)
    if err != nil {
      panic(err) // or consider as allowed
    }
    if !allow {
      return ResponseError429("over the limit");
    }

    //
    return ...
  })
}
```
