# Installation

```sh
go get github.com/shanbay/gobay/cmd/gobay
```

## gobay 项目辅助使用的工具

开发 gobay 项目前，可能需要先安装下面的这些工具，以保证各项功能的正常运行:
`或者也可以使用我们准备好的开发用的 docker image ，那里面已经安装好了这些工具`

- [golang 语言](https://golang.org/doc/install) [下载安装](https://golang.org/dl/)

- [OpenAPI API 文档工具](https://goswagger.io/install.html) [（介绍文档）](https://github.com/go-swagger/go-swagger)

```sh
# mac:
brew tap go-swagger/go-swagger
brew install go-swagger

# windows
# 这儿下载安装 release 版本
# https://github.com/go-swagger/go-swagger/releases/tag/v0.24.0
```

- [grpc 工具](https://github.com/grpc/grpc-go)

```sh
go get -u google.golang.org/grpc
```

- [protobuf 工具（grpc 需要）](https://github.com/golang/protobuf) [安装方法文档](http://google.github.io/proto-lens/installing-protoc.html)

```sh
# 安装 protobuf (或者至少安装 protobuf-compiler)
# mac:
brew install protobuf

go get -u github.com/gogo/protobuf/protoc-gen-gofast
go get -u github.com/golang/mock/mockgen
```

- [golang 的 lint 工具: golangci-lint](https://golangci-lint.run/usage/install/#local-installation)

```sh
# mac
brew install golangci/tap/golangci-lint
brew upgrade golangci/tap/golangci-lint

# windows
# 先安装git bash，再执行下面步骤（githubusercontent.com需要科学上网）
# binary will be $(go env GOPATH)/bin/golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.27.0
golangci-lint --version
```

- [ORM 工具: ent](https://github.com/shanbay/ent)

```sh
go get github.com/facebook/ent/cmd/entc@v0.4.0
```

- [测试工具: gotests](https://github.com/cweill/gotests)

```sh
go get -u github.com/cweill/gotests/
```

- [测试覆盖 tool: gocovmerge](https://github.com/wadey/gocovmerge)

```sh
go get github.com/wadey/gocovmerge
```

## Docker 开发用镜像

某些工具在某些系统里，安装可能会很困难。想要快速上手的话，你也可以使用我们的 docker 开发用镜像，略过这些安装步骤：

- 长期开发的话，还是建议你正常安装这些常用的 golang 工具。

使用方法：

1. 创建一个新项目（参考 [快速上手文档](quickstart_cn.md))
2. 项目里会有一个`dockerfiles`文件夹.
3. 进入 dockerfiles 文件夹，在里面运行 `sh run.sh`，就会启动一个 docker 开发用容器，可以在里面执行 gobay 的代码。

- docker 容器会 mount 当前的项目文件夹到容器里去，所以 IDE 修改本地文件夹的代码即可。
