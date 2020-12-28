# 接入 Message Queue

## config 配置

打开 config.yaml 文件，这几个配置

```yaml
  bus_broker_url: "amqp://guest:guest@rabbitmq:5672/"
  bus_exhanges:
    - some-exchange
  bus_queues:
    - hello.buses
    - world.buses
  bus_resend_delay: "1s"
  bus_publish_retry: 3
  bus_prefetch: 10
  bus_quit_consumer_on_empty_queue: false
  bus_bindings:
    - exchange: some-exchange
      queue: hello.buses
      binding_key: buses.a.hello
    - exchange: some-exchange
      queue: world.buses
      binding_key: buses.a.world
```

## 设置加载 mq(bus) 用的 extension

- `app/extensions.go`

```go
package app

import (
  "github.com/shanbay/gobay/extensions/busext"
)

func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    // ...
    "bus": &busext.BusExt{
      NS:          "bus_",
      ErrorLogger: custom_logger.NewSentryErrorLogger(), // 注意：加了这个后才能把错误报到sentry去
    },
    // ...
  }
}

var (
  // ...
  BusExt                     *busext.BusExt
  // ...
)

func InitExts(app *gobay.Application) {
  // ...
  BusExt = app.Get("bus").(*busext.BusExt)
  // ...
}
```

## 创建 mq 用的 server

创建 `app/bus` 文件夹，并在里面创建 `app/bus/server.go`（用于放mq的执行用代码） 和 `app/bus/handler.go`（用于放mq的处理逻辑）。

- `app/bus/server.go`

```go
package bus

import (
  myapp "git.17bdc.com/backend/helloworld/app"
  "github.com/shanbay/gobay"
)

func Serve(app *gobay.Application) error {
  myapp.InitExts(app)
  RegisterBusHandlers()

  err := myapp.BusExt.Consume()
  if err != nil {
    return err
  }
  return nil
}

func RegisterBusHandlers() {
  myapp.BusExt.Register("buses.a.hello", &HelloHandler{})
  myapp.BusExt.Register("buses.a.world", &WorldHandler{})
}

```

- `app/bus/handler.go`

```go
package bus

import (
  "context"
  "encoding/json"
  "log"
  "strconv"
  "time"

  "git.17bdc.com/backend/helloworld/app"
  "git.17bdc.com/backend/helloworld/app/models"
  "git.17bdc.com/backend/helloworld/app/services"
)

type HelloHandler struct {
  UserID        uint64 `json:"user_id"`
  DepartmentID  int    `json:"department_id"`
  // ... mq payload 里的其他内容
}

func (h *HelloHandler) ParsePayload(args []byte, kwargs []byte) (err error) {
  return json.Unmarshal(kwargs, h)
}

// 一般这儿会把主要逻辑包装出来，方便测试时用mock的代码替代
var PurchaseSuccessHandler func(ctx context.Context, userId uint64) error = services.HandlePurchaseSuccess

func (h *HelloHandler) Run() error {
  // 读取payload中的内容
  if h.DepartmentID != app.DEPARTMENT_ID {
    return nil
  }

  if err = PurchaseSuccessHandler(ctx, h.UserID); err != nil {
    return err
  }
  
  return nil
}
```

## 测试

```go
package bus

import (
  "context"
  "os"
  "strconv"
  "testing"
  "time"

  "git.17bdc.com/backend/helloworld/app"
  "git.17bdc.com/backend/helloworld/app/models"
  schema "git.17bdc.com/backend/helloworld/gen/entschema"
  protos_go "git.17bdc.com/shanbay/protos-go"
  "git.17bdc.com/shanbay/protos-go/xyz/oc"
  "github.com/golang/mock/gomock"
  "github.com/shanbay/gobay"

  "path"
)

func setup() *gobay.Application {
  // init app
  curdir, _ := os.Getwd()
  root := path.Join(curdir, "..", "..")
  extensions := app.Extensions()
  bapp, err := gobay.CreateApp(root, "testing", extensions)
  if err != nil {
    panic(err)
  }
  app.InitExts(bapp)
  // migrate db
  app.EntClient.Schema.Create(context.Background())

  return bapp
}

func tearDown() {
  // drop tables
  ctx := context.Background()
  app.EntClient.SampleDBModel.Delete().ExecX(ctx)

  redisclient := app.Redis.Client(context.Background())
  redisclient.FlushDB()
}

func TestHelloHandler_Run(t *testing.T) {
  setup()
  defer tearDown()

  // mock grpc 

  // 写入测试用的 db and redis data

  // 替换掉主要的逻辑 function，方便测试。
  PurchaseSuccessHandler = func(ctx context.Context, userId uint64) error {
    return nil
  }
  h := &OrderPaidHandler{
    UserID:       123,
    DepartmentID: app.DEPARTMENT_ID,
  }

  h.Run()
  
  // 检查 Run 的效果
  if currentState != expectedResult {
    t.Errorf("failed: expected %v, got %v", expectedResult, currentState)
  }
}
```
