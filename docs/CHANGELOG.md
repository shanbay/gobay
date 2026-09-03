# 1.2.10 (2026-09-03)

- **行为变更：四个 redis 扩展入口（`cachext` 的 v6/v9 backend、`redisext` 的 v6/v9）现在都带连接池与超时默认值**，业务项目无需任何配置即可生效：

  | 参数           | 默认值 | 配置键（v6 / v9）                         | go-redis 原默认值                 |
  | -------------- | ------ | ----------------------------------------- | --------------------------------- |
  | 连接池上限     | 100    | `<NS>poolsize`                            | `10 × NumCPU` / `10 × GOMAXPROCS` |
  | 读超时         | 200ms  | `<NS>readtimeout`                         | 3s                                |
  | 取连接排队超时 | 100ms  | `<NS>pooltimeout`                         | ReadTimeout + 1s                  |
  | 空闲回收       | 2m     | `<NS>idletimeout` / `<NS>connmaxidletime` | 5m / 30m                          |

- `PoolSize` 默认值由 1.2.9 的 20 提高到 **100**。20 过于保守，会限制正常流量下的连接需求；100 是按实测单池峰值取的兜底上限
- **新增三项超时默认值。** go-redis 的原默认值（读 3 秒、排队 4 秒）长于典型在线链路的上游预算，等于把请求堆积留在自己进程里。v6 尤其需要：它的连接读写与排队都不接受 `context`，上游超时管不到 redis 调用，这三项是唯一能限制单连接占用时长的闸。v9 虽然 `deadline` 取 `min(ctx.Deadline(), now+ReadTimeout)`，但 ReadTimeout 作为静态上界仍然必要——没有 deadline 的调用（离线任务、cronjob）否则会落在 3 秒上
- **空闲回收收敛到 2 分钟。** v9 的 `ConnMaxIdleTime` 默认 30 分钟，一次瞬时并发建出来的连接会被留到半小时后才回收，池子只涨不落
- 需要放宽的服务显式配置对应键即可覆盖；`<NS>poolsize: 0` 仍然回到 go-redis 自己的默认值
- `redisext` 补上首批带断言的测试（此前只有无断言的 Example 测试）

- **配置键名现在支持下划线写法**：`redis_poolsize` 与 `redis_pool_size` 都生效。mapstructure 只做大小写折叠、不做下划线归一化，此前后者会被静默忽略；新增的 `gobay.NormalizeUnderscoreKeys` 在 `Unmarshal` 前补齐。两种写法并存时以**不带下划线的**为准——交给 mapstructure 自行处理的话，哪个生效取决于 map 迭代顺序，是不确定的。

⚠️ 时间类参数必须带单位：`200ms` 正确，`200` 会被当成 200 纳秒。

⚠️ `redisext` 的 `<NS>host` 键**不生效**（`redis.Options` 只有 `Addr` 没有 `Host`，mapstructure 找不到字段会静默跳过，go-redis 随后 fallback 到 `localhost:6379`）。本版本未改动这一行为，请显式使用 `<NS>addr`。**`cachext` 不受影响**——它的 redis backend 有 `Addr` 为空时回落到 `<NS>host` 的兼容代码，`cache_host` 照常生效。

# 1.2.9 (2026-09-02)

- `cachext` 的两个 redis backend（v6 / v9）的 `Init` 从硬编码 `host`/`password`/`db` 三个字段改为 `config.Unmarshal`，现在 `redis.Options` 的全部字段都能通过配置设置，与 `redisext` 一致。常用的是 `<NS>poolsize` / `<NS>pooltimeout`
- **行为变更：`PoolSize` 默认值由 go-redis 的 `10 × NumCPU` 改为 20**。`NumCPU()` 读的是宿主机核数而非容器的 CPU limit，多核节点上的容器会拿到远超实际需要的池上限；这个上限会在 redis 变慢时变成放大器（请求堆积 → 建更多连接 → redis 更慢）。需要更大的池显式配置 `<NS>poolsize: <n>`；**要退回 go-redis 原默认值配 `<NS>poolsize: 0`**，不必回滚版本。未显式配置时启动日志会打印实际生效值与来源
- v9 backend 在 `GOMAXPROCS < NumCPU`（Go 1.25 的 container-aware GOMAXPROCS 生效，或引入了 automaxprocs）时**不再覆盖** go-redis 的默认值——那种环境下 `10 × GOMAXPROCS` 会随容器规格缩放，比固定值更合理。判断的是运行结果而非 `runtime.Version()`，因为 `containermaxprocs` GODEBUG 的默认值取决于主模块的 go 指令，依赖库读不到。v6 backend 用的是 `runtime.NumCPU()`，不受此影响，始终用固定默认值
- `<NS>addr` 作为 `<NS>host` 的等价键名（同时配置时 `addr` 优先）；两者都缺失时 `Init` 直接返回错误，而不是让 go-redis 静默 fallback 到 `localhost:6379`
- cachext 的 backend 初始化错误现在带上 NS 前缀，便于定位是哪个 CacheExt

⚠️ 配置键名**不能带下划线**：`cache_poolsize` 生效，`cache_pool_size` 会被静默忽略（mapstructure 只做大小写折叠，不做下划线归一化）。时间类参数必须带单位，`500ms` 正确，`500` 会被当成 500 纳秒。（下划线写法自 1.2.10 起已支持）

# 1.2.8 (2026-07-27)

- `asynctaskext`/`busext` 新增 Prometheus 处理耗时/QPS 埋点（`asynctask_task_duration_seconds`/`bus_task_duration_seconds`），config `<NS>monitor_enable` 开关可选开启，默认关闭零开销；与 Python coast 库同名同 label 同 buckets，可跨语言合并查询

# 1.2.7 (2026-06-16)

- 移除 shanbay/go-redis fork 的 replace 指令，改用官方 `github.com/redis/go-redis/extra/redisotel/v9` v9.18.0 及 `rediscmd/v9` v9.18.0
- 升级 go 版本至 1.24
- CI 矩阵新增 Go 1.24，更新 actions 版本（checkout@v4、setup-go@v5、golangci-lint-action@v6、cache@v4）

# 1.2.6 (2025-08-20)

- EntExt 新增 CheckHealth 方法

# 1.2.5 (2024-12-16)

- Tracing 基础设施同时保留 APM 和 OpenTelemetry 并由从 config 中读取配置改为从环境变量中读取

# 1.2.4 (2024-9-27)

- otelsql 在过滤 Span 时增加对 TraceFlags 是否采样的判断

# 1.2.3 (2024-9-26)

- 新增 busext 的 HealthCheck 方法

GRPC 和 OpenAPI 的 health check 检查时，可以添加检查 BusExt。

```go
func (s *myServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if req.Service == "liveness" || req.Service == "readiness" {
		if app.BusExt != nil {
			if err := app.BusExt.HealthCheck(); err != nil {
				return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
			}
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	}
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_UNKNOWN}, nil
}
```

# 1.2.2 (2024-7-23)

- 对 otelsql 设置 `DisableErrSkip: true` 以忽略 ErrSkip

# 1.2.1 (2024-5-17)

- 新增 ISeqRedis 抽象执行 Lua 脚本，隐藏 go-redis 版本差异

# 1.2.0 (2024-5-1)

- 升级 go1.21
- 添加用于执行定时任务的 CronJobExt
- 新增 go-redis/v9 作为可选升级项
- 将 APM 替换成 OpenTelemetry 实现

# 1.0.0 (2022-01-05)

- 删除 cachext 的 cacheNil 逻辑

**BREAKING CHANGE**:

- 去掉 WithCacheNil 配置
- 以前判断了 cachext.Nil 的地方，应该改为判断响应后 out 值是否为空值

# 0.16.3 (2021-10-09)

- 修正 `DialOptions` 拼写
- 修复启用 apm 的时候 retry 不生效的问题

**BREAKING CHANGE**:

初始化 stubext 时参数应该改叫 `DialOptions`

# 0.16.0 (2021-08-16)

- 改用 `shanbay/amqp`
- 增加 asynctask ext 的健康检查
  - 使用方法：`curl 127.0.0.1:9000/health?timeout=5&queue=gobay.task_sub`

# 0.14.0 (2020-11-19)

- sentryext 收集当前栈信息，让 sentry web 界面上可以展开
- redisext 支持更多可配置项

**BREAKING CHANGE**:

redisext 的 `host` 配置需要改名为 `addr` （注意是 redisext 而不是 cachext）

> 如果你发现没修改也正常运行了，可能是未读到使用了默认值 `localhost:6379`

# 0.13.10 (2020-11-03)

- 增强 health check

GRPC 和 OpenAPI 的 health check 检查时，可以添加检查 Cache, Redis, 每个 DB。

```go
func (h *luneServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if req.Service == "liveness" || req.Service == "readiness" {
		if app.EntExt != nil {
			if err := app.EntClient.CheckHealth(ctx); err != nil {
				return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
			}
		}

		if app.Redis != nil {
			if err := app.Redis.CheckHealth(ctx); err != nil {
				return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
			}
		}

		if app.Cache != nil {
			if err := app.Cache.CheckHealth(ctx); err != nil {
				return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
			}
		}

		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	}
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_UNKNOWN}, nil
}
```

# 0.13.9 (2020-10-16)

- 添加 openapi 的 ent 报错处理 middleware 。 ent 报错后，把错误 panic 出来，可以自动处理 404 not found 和 400 constraint error。

```
// 已经创建的项目可以添加这行添加
// app/openapi/server.go
// 添加:
mdwBuilders = append(mdwBuilders, entopenapimw.GetEntMw(myapp.EntExt))

// 在这个之前
s.SetHandler(gmw(api.Serve(openapi.ChainMiddlewares(...
```

- 添加方便写测试的 testhelper ，详情参考 [writing_test.md](https://github.com/shanbay/gobay/blob/master/docs/writing_test.md)

# 0.13.6 (2020-09-29)

- cache ext 的 GetMany 会把未命中的 key 对应的 interface{} 置为 nil

**BREAKING CHANGE**:

项目需要修改 GetMany 后是否命中的判定方法，改为判断值是否为 nil

# 0.12.11 (2020-08-28)

- 更新 ent 版本到 0.4.0

**BREAKING CHANGE**:

需要修改项目里的 ent 版本：

1. 更新 dependnecy - go.mod

```
github.com/facebookincubator/ent
// 这行，改为
entgo.io/ent v0.4.0

// 并把下面的 replace 改为
replace entgo.io/ent => github.com/shanbay/ent v0.4.0
```

2. 处理所有 \*.go 代码

```
// 替换所有的： /facebookincubator/ent => /facebook/ent
```

3. 更新 generate ent 用的 template

```
//
// spec/enttmpl/builder_create - 删除
// spec/enttmpl/builder_query - 修改，参考[这个](/cmd/gobay/templates/spec/enttmpl/builder_query.tmpl)
// spec/enttmpl/client - 改为，参考[这个](/cmd/gobay/templates/spec/enttmpl/client.tmpl)
// spec/enttmpl/sql_create - 删除
```

4. 检查 ent 生成的 mysql enum，可能出现 `StatusREFUNDFAILED` 需要改成 `StatusREFUND_FAILED` 的问题。

5. 跑测试看看有没有其他问题。
