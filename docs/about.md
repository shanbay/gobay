# 关于 Gobay

`Gobay` 由 [扇贝](https://www.shanbay.com) 的后端团队开发，是一个 [Golang](https://golang.org/) 的微服务框架。支持 [gRPC](https://grpc.io/) 和 HTTP(OpenAPI) 接口。

需要编写完整的基于 `gRPC` 或 `OpenAPI` 的微服务，除了接口层面，还有大量的业务逻辑需要编写。开发`Gobay`的目的正是为了让大家可以更方便的编写这些代码。

`Gobay` 采用“扩展(Extention)”架构，核心是一个配置和生命周期管理。我们提供了内置的一些扩展，包括 ORM，缓存，异步任务，异常报警等。
