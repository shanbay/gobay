# 异步任务

有些 function 可能需要执行一段时间，而 RPC 和 API 分别有自己的 timeout 超时设置。我们需要用async task来运行这些不能在一个请求内完成的任务。

比如：请求某个RPC服务后，我们需要去调用一个微信的API，发出通知。微信的API请求可能需要500ms才能完成，但我们的RPC服务500ms就已经超时了。这种情况下，就应该把"调用微信"的任务，抛出给async task去完成，RPC的线程则尽早完成RPC请求。

- Async task 需要使用redis作为传递数据的媒介。
- 生产环境下，需要单独启动一个pod/服务，用于接受和处理这些异步任务。

```sh
make run COMMAND=asynctask
```


## config 配置

首先，配置config.yaml

```yaml
  asynctask_concurrency: 10
  asynctask_broker: "redis://redis:6379/8"
  asynctask_default_queue: "gobay.task"
  asynctask_result_backend: "redis://redis:6379/8"
  asynctask_results_expire_in: 600
  asynctask_redis: {}
```

## 设置加载时用的 extension

- `app/extensions.go`

```go
package app

import (
  "github.com/shanbay/gobay/extensions/asynctaskext"
)

func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    // ...
    "asyncTask": &asynctaskext.AsyncTaskExt{NS: "asynctask_"},
    // ...
  }
}

var (
  // ...
  AsyncTask                  *asynctaskext.AsyncTaskExt
  // ...
)

func InitExts(app *gobay.Application) {
  // ...
  AsyncTask = app.Get("asyncTask").Object().(*asynctaskext.AsyncTaskExt)
  // ...
}
```

## 准备接受 async task 的处理

- `app/asynctask/server.go`

```go
package asynctask

import (
  myapp "git.17bdc.com/backend/sample/app"
  "github.com/shanbay/gobay"
)

func Serve(app *gobay.Application) error {
  myapp.InitExts(app)
  RegisterAsyncTaskWorkerHandlers()

  if err := myapp.AsyncTask.StartWorker("", 10); err != nil {
    return err
  }
  return nil
}

func RegisterAsyncTaskWorkerHandlers() {
  if err := myapp.AsyncTask.RegisterWorkerHandlers(map[string]interface{}{
    "SomeAsyncTask": SomeAsyncTask,
  }); err != nil {
    panic(err)
  }
}
```

- 并在 `app/asynctask/handler.go` 里编写处理task用的逻辑代码。

```go

func SomeAsyncTask(ctx context.Context, relatedItemIDToProcess uint64) error {
  // 逻辑代码

  return nil
}
```

## 触发启动 async task

```go
  // start async task
  sign := &tasks.Signature{
    Name: "SomeAsyncTask",
    Args: []tasks.Arg{
      {Type: "uint64", Value: sample.ID},
    },
  }
  _, err = app.AsyncTask.SendTask(sign)
  if err != nil {
    panic(err)
  }
```

## 测试代码只需测试 SomeAsyncTask 函数本身即可
