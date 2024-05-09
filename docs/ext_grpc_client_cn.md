# 调用别的 GRPC 接口

## config 配置

对于每个要调用的GRPC服务，都需要在 config.yaml 里配置上这些

比如，要连接 `rpcsvc` 这个服务：

```yaml
  stub_rpcsvc_host: "127.0.0.1"
  stub_rpcsvc_port: 6000
  stub_rpcsvc_authority: "rpcsvc-rpc.xyz"
  stub_rpcsvc_metadata:
    auth_token: "abcdefg"
  stub_rpcsvc_retrybackoff: 50ms
  stub_rpcsvc_retrytimes: 3
  stub_rpcsvc_conntimeout: 5s
```

- stub_rpcsvc_host: 要调用的 RPC 所在的 host
- stub_rpcsvc_port: 要调用的 RPC 所在的端口
- stub_rpcsvc_authority: RPC路由信息
- stub_rpcsvc_metadata: 这个根据 RPC 服务的要求，随意。
最后三个默认的配置基本够用。

## 加载时准备 GRPC 调用服务

- `app/extensions.go`

```go
package app

import (
  schema "git.17bdc.com/backend/helloworld/gen/entschema"
  "entgo.io/ent/dialect"
  _ "github.com/go-sql-driver/mysql"
  "github.com/shanbay/gobay"
  "github.com/shanbay/gobay/extensions/entext"
)

func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    // ...
    "stubRpcsvc": &stubext.StubExt{
      NS:          "stub_rpcsvc_",
      DailOptions: []grpc.DialOption{grpc.WithInsecure(), grpc.WithBlock()},
      NewClientFuncs: map[string]stubext.NewClientFunc{
        "rpcsvc": func(conn *grpc.ClientConn) interface{} {
          return xyz.NewRpcsvcClient(conn)
        },
      },
    },
    // ...
  }
}

var (
  // ...
  StubRpcsvc             *stubext.StubExt
  // ...
)

func InitExts(app *gobay.Application) {
  // ...
  StubRpcsvc = app.Get("stubRpcsvc").Object().(*stubext.StubExt)

  // 对于这个function的调用，具体请看下面 app/stub.go 里的内容
  setupClients(app.Env())
  // ...
}
```

- `app/stub.go`

```go
var (
  RpcsvcClient            xyz.RpcsvcClient
)

func setupClients(appEnv string) {
  var ok bool
  isNotTestingEnv := appEnv != "testing"
  if RpcsvcClient, ok = StubRpcsvc.Clients["rpcsvc"].(xyz.RpcsvcClient); !ok && isNotTestingEnv {
    // it's okay to appear in testing env
    log.Printf("RpcsvcClient init failed - client: %v, ok: %v", RpcsvcClient, ok)
  }
}

func RpcsvcCheck(ctx context.Context, userId uint64) (bool, error) {
  if checkRpcsvc, err := RpcsvcClient.CheckRpcsvc(
    StubRpcsvc.GetCtx(ctx),
    &xyz.CheckRpcsvcRequest{
      UserId:   userId,
    },
  ); err != nil {
    return false, err
  } else {
    return checkRpcsvc.Allow, nil
  }
}
```

## 测试时 mock 要调用的 GRPC 服务

```go
func mockOrderUpdateExp(t *testing.T) *gomock.Controller {
  ctrl := gomock.NewController(t)
  mockedClient := mock_oc.NewMockOrderClient(ctrl)
  app.OrderClient = mockedClient

  mockedClient.EXPECT().UpdateOrderExpress(
    gomock.Any(), gomock.Any(), gomock.Any(),
  ).Return(
    &oc.OrderObject{}, nil,
  ).AnyTimes()
  return ctrl
}

func tearDownMock(ctrl *gomock.Controller) {
  ctrl.Finish()
}
```
