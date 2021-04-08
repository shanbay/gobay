# 项目结构

```sh
.
├── Makefile # 常用命令
├── app # 写代码的地方
│   ├── asynctask # 异步任务服务。
│   │   ├── handlers.go
│   │   └── server.go
│   ├── constant.go # 设定常量
│   ├── extensions.go # 配置extension (redis/mysql/grpc/cache/sentry/elasicApm/etc.)
│   ├── grpc # GRPC 服务
│   │   ├── handlers.go
│   │   └── server.go
│   ├── models # DB model 和 cache 缓存
│   │   ├── cache.go
│   │   └── common.go
│   │   # ...可以添加更多model
│   └── oapi # API 服务
│       ├── handlers.go
│       └── server.go
├── cmd # 定义命令
│   ├── actions
│   │   ├── asynctask.go
│   │   ├── grpcsvc.go
│   │   ├── health_check.go
│   │   ├── oapisvc.go
│   │   └── root.go
│   │   # ...可以添加更多命令
│   └── main.go
├── config.yaml # 项目配置的 yaml 文件
├── gen # 生成的代码
├── go.mod # dependency
└── spec # 定义文件，会被用来生成 /gen 文件夹里的生成文件
    ├── enttmpl
    │   ├── # ent用的模板文件，不用改，定期升级即可
    ├── schema # ent 的数据库 schema 定义文件
    ├── grpc # grpc proto 文件
    ├── oapi
    │   └── main.yml # openapi v3 文档
```
