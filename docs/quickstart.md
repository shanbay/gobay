# Quick Start

```sh
gobay new <your-project>

# i.e.
gobay new github.com/company/helloworld-project
cd helloworld-project
```

- Run the tools in docker dev box if you like:

```sh
cd dockerfiles
sh run.sh
```

---

## To start a GRPC server

```sh
make run COMMAND=grpcsvc
```

then `grpc.health.v1.health/check` is available on port 6000.

### Add more GRPC handlers

1. Create your GRPC proto files into `spec/grpc` directory, i.e. `spec/grpc/helloworld.proto`

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

1. Generate code for protos

```sh
# using proto files in spec/grpc directory, generate protobuf go file.
make genproto

# using generated protobuf go file, generate mock protobuf go file for testing.
make genprotomock
```

1. open `app/grpc/server.go`, register api server in `func configureAPI() {...}` method.

```go
// i.e.
func configureAPI(s *grpc.Server, impls *helloworldProjectServer) {
  // ...
  // protos.RegisterHelloworldProjectServer(s, impls)
  grpc_health_v1.RegisterHealthServer(s, impls)
  // ...
}
```

2. open app/grpc/handlers.go, and implement the grpc method handler inside.

---

## To start an API server: New (oapi-codegen + echo)

> Only support OpenAPI v3

1. We need to generate some openapi code with the openapi spec (`spec/oapi/main.yml`) (require openapi tool, use docker dev box if needed)

```sh
# generate code
make genswagger
# go.mod
make tidy ensure
```

2. Start the server

```sh
make run COMMAND="oapisvc" ARGS="--env development"
```

Then you may view the api docs at [http://127.0.0.1:5000/helloworld-project/apidocs](http://127.0.0.1:5000/helloworld-project/apidocs)

### Add more API handlers

1. Update `spec/openapi/main.yml` (add more API specs)

2. generate code for openapi spec

```sh
make genswagger
```

3. Open `app/oapi/handlers.go` and add a new handler and logic code (implementing `ServerInterface` in `gen/oapi/oapi.go`)
