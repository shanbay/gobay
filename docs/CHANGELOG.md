# 0.15.0

- 升级到 go 1.16

**BREAKING CHANGE**:

要升级此版本，必须将项目的 go 版本升级到 1.16（开发、测试、打包环境需要升级 go 版本；修改 go.mod 文件）

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
github.com/facebook/ent v0.4.0

// 并把下面的 replace 改为
replace github.com/facebook/ent => github.com/shanbay/ent v0.4.0
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
