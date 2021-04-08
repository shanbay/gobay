# Project Structure

```sh
.
├── Makefile # common commands
├── app # core code area, implementations will be done in this directory.
│   ├── asynctask # async job (server). You can use async job extension client, create a job into a redis queue, and let the async job server handle the queue.
│   │   ├── handlers.go
│   │   └── server.go
│   ├── constant.go # define constants here
│   ├── extensions.go # define and prepare extensions (redis/mysql/grpc/cache/sentry/elasicApm/etc.)
│   ├── grpc # GRPC service controllers
│   │   ├── handlers.go
│   │   └── server.go
│   ├── models # repository for models and cache
│   │   ├── cache.go
│   │   └── common.go
│   │   # ...add more models and repositories here
│   └── oapi # API
│       ├── handlers.go
│       └── server.go
├── cmd # command entry, start a (one of) grpc/openapi/asynctask/etc server by calling a command here.
│   ├── actions
│   │   ├── asynctask.go
│   │   ├── grpcsvc.go
│   │   ├── health_check.go
│   │   ├── oapisvc.go
│   │   └── root.go
│   │   # ...add more command
│   └── main.go
├── config.yaml # configs in yaml file
├── gen # geneated code, i.e. openapi/protobuf(grpc)/ent(mysql), populated with corresponding tools
├── go.mod # dependency
└── spec # specification for tools to generate stuff in /gen directory (i.e. proto for protobuf, yaml for openapi, tmpl/schema for ent)
    ├── enttmpl
    │   ├── builder_create.tmpl
    │   ├── builder_query.tmpl
    │   ├── client.tmpl
    │   └── sql_create.tmpl
    ├── schema # schema directory for (ent) db models
    ├── grpc # proto files here
    ├── oapi
    │   └── main.yml # openapi v3 spec
    # ...add more if needed here
```
