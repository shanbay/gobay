# 快速开始

- 安装 gobay 命令行工具

```sh
go get github.com/shanbay/gobay/cmd/gobay
```

- 使用 gobay 命令行工具生成新项目

```sh
gobay new <your-project>

# i.e.
gobay new github.com/company/helloworld-project
cd helloworld-project
```

- 如果有需要，可以使用准备好的 docker 开发用镜像，略过安装步骤快速开始使用。

```sh
cd dockerfiles
sh run.sh
```

---

## 开启 GRPC 服务

```sh
make run COMMAND=grpcsvc
```

开启后，至少 `grpc.health.v1.health/check` 将会在 6000 端口可用.

### 添加更多的 GRPC 服务

1. 在`spec/grpc`文件夹里，创建你的 proto 文件,

比如 `spec/grpc/helloworld.proto`

```proto
syntax = "proto3";

package helloworld;
option go_package = "github.com/com/example/helloworld";

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply) {}
}
message HelloRequest {
  string name = 1;
}
message HelloReply {
  string message = 1;
}
```

1. 生成 proto 用的 golang 代码

```sh
# using proto files in spec/grpc directory, generate protobuf go file.
make genproto

# using generated protobuf go file, generate mock protobuf go file for testing.
make genprotomock
```

1. 打开 `app/grpc/server.go`, 在 `func configureAPI() {...}` function 中注册你的 proto 用的 GRPC API 服务.

```go
// i.e.
func configureAPI(s *grpc.Server, impls *helloworldProjectServer) {
  // 添加
  protos.RegisterHelloworldProjectServer(s, impls)

  grpc_health_v1.RegisterHealthServer(s, impls)
  // ...
}
```

1. 打开 `app/grpc/handlers.go`, 在里面编写你的 grpc 服务代码。

---

## 开启一个 API 服务

1. 首先要先写一些符合 openapi 规范的 API 定义文档 (`spec/openapi/main.yml`)

1. 使用 openapi 定义文档，生成模板代码（需要使用 openapi 工具，没有安装的话，docker 开发用镜像里已经把它准备好了）

```sh
make genswagger
```

1. 启动 API 服务

```sh
make run COMMAND=httpsvc
```

这时你可以在 [http://127.0.0.1:5000/helloworld-project/docs](http://127.0.0.1:5000/helloworld-project/docs) 查看 OpenAPI 的文档了。

### 添加更多的 API

1. 更新 `spec/openapi/main.yml` API 文档文件

1. 重新生成模板代码

```sh
make genswagger
```

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
