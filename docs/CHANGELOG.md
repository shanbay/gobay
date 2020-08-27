# 0.12.11 (2020-08-28)

- 更新 ent 版本到 0.4.0

BREAKING CHANGE:
需要修改项目里的 ent 版本：

```go
// go.mod -------- (dependency)

github.com/facebookincubator/ent
// 这行，改为
github.com/facebook/ent v0.4.0

// 并把下面的 replace 改为
replace github.com/facebook/ent => github.com/shanbay/ent v0.4.0

// 所有 *.go 代码 --------- (golang 代码)
// 替换所有的： /facebookincubator/ent => /facebook/ent

// spec/enttmpl -----------(ent 模板)
// spec/enttmpl/builder_create - 删除
// spec/enttmpl/builder_query - 改为
// spec/enttmpl/client - 改为
// spec/enttmpl/sql_create - 删除


```
