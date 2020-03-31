# 如何使用

## 生成项目文件夹

```bash
# go get github.com/shanbay/gobay/cmd/gobay
gobay new github.com/shanbay/my_proj
```

## 可能需要安装的一些二进制

- openapi工具: [go-swagger](https://github.com/go-swagger/go-swagger/releases)
- grpc/protobuf工具: [grpc-go](https://github.com/grpc/grpc-go) [protobuf](https://github.com/golang/protobuf)
- lint工具: [golangci-lint](https://github.com/golangci/golangci-lint#binary)
- orm代码生成(ent): [ent](https://github.com/shanbay/ent)
- pre-commit: [pre-commit](https://pre-commit.com/#installation)
- gotests: [gotests](https://github.com/cweill/gotests)

## 初始化

在目录中 `git init` 并 `pre-commit install` 来注册hooks，它会在每次commit前检查代码格式和语法。

## 目录结构

### app目录

核心代码，包含各个server的启动和handler代码。以及models等部分的代码。大部分场景下你需要在这个目录下编写代码。

### cmd目录

程序入口，`gobay` 可以通过参数指定运行 `grpc openapi asynctask` 等多种 server 中的一个
如果需要增加新的server类型，请在`cmd`目录下增加新的入口。

### spec目录

一些扩展会生成一部分代码，这些扩展依赖某些源文件（如protobuf的proto文件、openapi的yaml文件、ent的tmpl sehema文件等）。作为约定，这些源文件统一放在`spec`目录下。

### gen目录

一些扩展会生成一部分代码，比如`openapi, protobuf, ent`等，作为约定这些生成的代码统一保存在`gen`目录里。如果用到这些扩展需要使用相应的`openapi, grpc, ent`工具来生成。

## 生成测试

使用 `gotests` 为已经写好的代码生成测试，详见 `gotests --help` 

## Makefile 中的常用功能

### make clean

清理临时目录

### make build_debug

编译用于debug的二进制到 `.build` 目录

### make run

debug模式运行代码

### make build_release

编译用于生产环境的二进制到 `.dist` 目录

### make test

运行测试

### make coverage

运行测试并查看覆盖率文件

### make coverage_fail

运行测试并检查覆盖率是否达标（Makefile中的 `COVERAGE_FAIL_UNDER` 变量）

### make fmt

format代码

### make lint

lint检查

### make tidy

等同于`go mod tidy`

### make ensure

等同于`go mod download`

### make genswagger

根据Makefile中的 `SWAGGER_SPEC`（默认为spec/openapi/main.yml）来生成代码到gen目录

### make entinit

用法：`make ARGS="UserBook TableName TableNameTwo" entinit`

会生成model的schema到spec/schema目录，需要将它补充完整。详见[ent quick start](https://entgo.io/docs/getting-started/)。

### make entdesc

预览schema会生成的数据库结构

### make entgen

根据schema生成代码到gen目录

# 贡献代码

## 更新 templates

更新完静态文件（templates目录），需要将静态文件打包成.go文件：

```bash
# go get github.com/markbates/pkger/cmd/pkger
pkger -include /cmd/gobay/templates -o cmd/gobay/
```
