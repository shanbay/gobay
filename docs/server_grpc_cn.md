# 使用 GRPC

## 检查 config 文件

检查一下项目下的 config.yaml 文件，应该有这几行

```yaml
  grpc_listen_host: 0.0.0.0
  grpc_conn_timeout: 1s
```

RPC默认监听6000端口的请求。可以添加这个配置来修改：

```yaml
  grpc_listen_port: 6000
```

## 准备好你的 proto 文件

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

*注意proto文件中的`option go_package = "github.com/com/example/helloworld";`

这个package的名字需要引入到你的golang代码里去。

1. 生成 proto 用的 golang 代码

```sh
# using proto files in spec/grpc directory, generate protobuf go file.
make genproto

# using generated protobuf go file, generate mock protobuf go file for testing.
make genprotomock
```

## 给 proto 文件写执行逻辑

1. 打开 `app/grpc/server.go`, 在 `func configureAPI() {...}` function 中注册你的 proto 用的 GRPC API 服务.

```go
import "github.com/com/example/helloworld"

// i.e.
func configureAPI(s *grpc.Server, impls *helloworldServer) {
  // 添加你的新 proto 的 server
  helloworld.RegisterGreeterServer(s, impls)

  // 保留基础的 health check
  grpc_health_v1.RegisterHealthServer(s, impls)
  // ...
}
```

1. 打开 `app/grpc/handlers.go`, 在里面编写你的 grpc 服务代码。

```go
import "github.com/com/example/helloworld"

func (s *helloworldServer) Greeter(ctx context.Context, req *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
  // your logic here
  return &helloworld.HelloReply{Message: "pong"}, nil
}
```


## 开启 GRPC 服务

```sh
make run COMMAND=grpcsvc
```

开启后，GRPC 服务将会在 6000 端口（默认）可用.


## 给 GRPC 服务写测试

请参考出门右转的 [写测试](writing_test_cn.md)
