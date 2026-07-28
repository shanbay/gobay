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

## 监控指标

在 config.yaml 里加一行即可开启处理耗时/QPS 埋点（默认关闭，零开销）：

```yaml
  bus_monitor_enable: true
```

开启后会记录 Prometheus Histogram 指标 `bus_task_duration_seconds`（label：`task_name`、`queue`、`status`；两者都取消息的 routing key，因为 busext 目前没有独立于 routing key 的"任务名"概念）。`status` 是真实的处理结果：`success` / `failure`（`handler.Run()` 返回 error）/ `parse_error`（payload 解析失败）/ `invalid_message`（headers、content-type、encoding、未注册 routing key 等消息本身有问题）。gobay 本身**不会**自建 `/metrics` HTTP server，需要使用方应用自己起一个（如 `promhttp.Handler()`），照搬 `cachext` 的既有惯例。

这个指标和 Python coast 库（`coast/celery.py`）里的 `bus_task_duration_seconds` 是**同一个指标**（同名、同 label、同 buckets），可以在同一个 Prometheus/Grafana 查询里合并 Go 和 Python 两边的数据，不需要额外处理。

常用 PromQL：

```promql
# QPS
sum(rate(bus_task_duration_seconds_count[1m])) by (task_name)

# 失败率
sum(rate(bus_task_duration_seconds_count{status="failure"}[5m])) by (task_name)
  / sum(rate(bus_task_duration_seconds_count[5m])) by (task_name)

# P95 耗时
histogram_quantile(0.95, sum(rate(bus_task_duration_seconds_bucket[5m])) by (le, task_name))
```
