# 0.12.12 (2020-10-16)

- 添加 openapi 的 ent 报错处理 middleware 。 ent 报错后，把错误 panic 出来，可以自动处理 404 not found 和 400 constraint error。

```
// 已经创建的项目可以添加这行添加
// app/openapi/server.go
// 添加:
mdwBuilders = append(mdwBuilders, entopenapimw.GetEntMw(myapp.EntExt))

// 在这个之前
s.SetHandler(gmw(api.Serve(openapi.ChainMiddlewares(...
```

# 0.12.11 (2020-08-28)

- 更新 ent 版本到 0.4.0

BREAKING CHANGE:
需要修改项目里的 ent 版本：

1. 更新 dependnecy - go.mod

```
github.com/facebookincubator/ent
// 这行，改为
github.com/facebook/ent v0.4.0

// 并把下面的 replace 改为
replace github.com/facebook/ent => github.com/shanbay/ent v0.4.0
```

2. 处理所有 \*.go 代码

```
// 替换所有的： /facebookincubator/ent => /facebook/ent
```

3. 更新 generate ent 用的 template

```
//
// spec/enttmpl/builder_create - 删除
// spec/enttmpl/builder_query - 修改，参考[这个](/cmd/gobay/templates/spec/enttmpl/builder_query.tmpl)
// spec/enttmpl/client - 改为，参考[这个](/cmd/gobay/templates/spec/enttmpl/client.tmpl)
// spec/enttmpl/sql_create - 删除
```

4. 检查 ent 生成的 mysql enum，可能出现 `StatusREFUNDFAILED` 需要改成 `StatusREFUND_FAILED` 的问题。

5. 跑测试看看有没有其他问题。
