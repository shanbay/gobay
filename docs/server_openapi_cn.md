# 使用 OpenAPI

## 检查 config 文件

检查一下项目下的 config.yaml 文件，应该有这几行

```yaml
  openapi_listen_host: "0.0.0.0"
  openapi_listen_port: 5000
```

这让API默认监听5000端口的请求。

## 准备好你的 OpenAPI spec 文件

1. 在`spec/grpc`文件夹里，创建你的 proto 文件,

首先要先写一些符合 openapi 规范的 API 定义文档，写在 `spec/openapi/main.yml` 里:

```yaml
swagger: "2.0"
info:
  title: ""
  description: ""
  version: 1.0.0
consumes:
  - application/json
produces:
  - application/json
schemes:
  - http
basePath: /helloworld
definitions:
  BaseDefaultRes:
    type: object
  BadRequestRes:
    type: object
    properties:
      msg:
        type: string
  SampleReq:
    type: object
    properties:
      sample:
        type: string
  SampleResponse:
    type: object
    properties:
      result:
        type: string
paths:
  /my-api-route:
    post:
      tags:
        - health
      operationId: apiOperationName
      summary: an api operation
      parameters:
        - in: query
          name: type
          type: string
        - in: body
          name: body
          required: true
          schema:
            $ref: "#/definitions/SampleReq"
      responses:
        "200":
          description: Success
          schema:
            $ref: "#/definitions/SampleResponse"
        "404":
          description: "unknown check type"
          schema:
            $ref: "#/definitions/BadRequestRes"
```

关键点：

- operationId: 这是写API时，你将具体编写代码的 function 名字。
- tags: 最好只用一个 tag，这个 tag 会成为 go package 的名字。

1. 使用 openapi 定义文档，生成模板代码（需要使用 openapi 工具，没有安装的话，docker 开发用镜像里已经把它准备好了）

```sh
make genswagger
```

## 给 API spec 文件写执行逻辑

1. 打开`openapi/server.go`, 在 `func configureAPI() {...}` function 中添加新的 api handler

```go
// i.e.
func configureAPI(s *restapi.Server, api *operations.HelloworldProjectAPI, impls *helloworldProjectServer, enableApm bool) {
  // ...
  api.HealthHealthCheckHandler = impls.healthCheckHealthHandler()
  // ...
}

```

1. 打开 `app/openapi/handlers.go`，在里面添加新的 handler 的逻辑代码

```go
func (s *helloworldProjectServer) healthCheckHealthHandler() health.HealthCheckHandler {
  return health.HealthCheckHandlerFunc(func(params health.HealthCheckParams) middleware.Responder {
    if params.Type == nil {
      return health.NewHealthCheckNotFound()
    }
    check_type := *params.Type
    if check_type == "liveness" || check_type == "readiness" {
      return health.NewHealthCheckOK()
    } else {
      return health.NewHealthCheckNotFound()
    }
  })
}
```

## 开启 API 服务

```sh
make run COMMAND=httpsvc
```

这时你可以在 [http://127.0.0.1:5000/helloworld-project/docs](http://127.0.0.1:5000/helloworld-project/docs) 查看 OpenAPI 的文档了。

## 给 API 服务写测试

请参考出门右转的 [写测试](writing_test_cn.md)
