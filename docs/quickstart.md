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

1. open app/grpc/handlers.go, and implement the grpc method handler inside.

---

## To start an API server

1. We need to generate some openapi code with the openapi spec (`spec/openapi/main.yml`) (require openapi tool, use docker dev box if needed)

```sh
make genswagger
```

1. Start the server

```sh
make run COMMAND=httpsvc
```

Then you may view the api docs at [http://127.0.0.1:5000/helloworld-project/docs](http://127.0.0.1:5000/helloworld-project/docs)

### Add more API handlers

1. Update `spec/openapi/main.yml` (add more API specs)

1. generate code for openapi spec

```sh
make genswagger
```

1. open `openapi/server.go`, register grpc server in `func configureAPI() {...}` method.

```go
// i.e.
func configureAPI(s *restapi.Server, api *operations.HelloworldProjectAPI, impls *helloworldProjectServer, enableApm bool) {
  // ...
  api.HealthHealthCheckHandler = impls.healthCheckHealthHandler()
  // ...
}

```

1. open `app/openapi/handlers.go`, and implement the api method handler inside
