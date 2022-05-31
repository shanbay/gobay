# 定时任务 cronjob

通常我们会使用 kubernetes 的 Cronjob 来执行定时任务。但 k8s 的 Cronjob 存在执行调度粒度的问题，最小只支持到**分钟**级别。

因此 gobay 提供了支持调度间隔粒度为秒级的 CronJobExt。

CronJobExt 的主要特点：

- CronJobExt 需要使用和 asynctaskext 相同的配置，或者可以直接复用其配置
- 需要单独启动一个程序或者容器用于发送这些任务给异步任务队列，功能和行为上类似 `celery beat`

## 适用场景

- 需要的调度间隔不满足整数分钟的场合，比如每90秒运行一次
- 间隔较小且需要频繁运行的任务，比如每1分钟拉取一次数据的任务等等

## config 配置

首先，配置 config.yaml

```yaml
  cronjob_concurrency: 10
  cronjob_broker: "redis://redis:6379/8"
  cronjob_default_queue: "gobay.task"
  cronjob_result_backend: "redis://redis:6379/8"
  cronjob_results_expire_in: 600
  cronjob_redis: {}
  cronjob_tz: "Asia/Tokyo"  # 默认为 UTC
  cronjob_health_check_port: 8080  # 默认为5000
```

如果想要复用 async task 的配置（比如需要像某个特定的任务队列发送定时任务）：

```yaml
cronjob_reuse_other: "other_asynctask_"  # 复用other_asynctask的配置，other_asynctask_为配置项的前缀
cronjob_health_check_port: 5001
```

## 设置加载时用的 extension

- `app/extensions.go`

```go
package app

import (
  "github.com/shanbay/gobay/extensions/cronjobext"
)

func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    // ...
    "cronJob": &cronjobext.CronJobExt{NS: "cronjob_"},
    // ...
  }
}

var (
  // ...
  CronJob                  *cronjobext.CronJobExt
  // ...
)

func InitExts(app *gobay.Application) {
  // ...
	CronJob = app.Get("cronJob").Object().(*cronjobext.CronJobExt)
  // ...
}
```

## 设置asynctask ext

需要一个 worker 来执行定时任务，CronJobExt 只发送任务不运行这些任务。

请参考：[配置asynctask ext](./ext_asynctask_cn.md)

## 使用

确保所以要注册的定时任务使用的 `TaskFunc` 都已经注册到了 AsyncTaskExt 里。

任务需要包装成 `CronJobTask` 类型，然后使用注册接口进行注册：

```go
var cronjobs = map[string]*CronJobTask{
    "sub": {
        Type:     DurationScheduler,  // 支持time.ParseDuration格式的时间间隔字符串
        Spec:     "1s",   // 调度间隔
        TaskFunc: TaskSub,  // 执行任务的函数，需要被注册到async task中
        TaskSignature: &tasks.Signature{
            Name: "sub_cron",  // TestFunc 被注册到async task时的名字，两者必须一致
            Args: []tasks.Arg{
                {
                    Name:  "arg1",
                    Type:  "int64",
                    Value: int64(1234),
                },
                {
                    Name:  "arg2",
                    Type:  "int64",
                    Value: int64(4321),
                },
            },
        },
    },
    "fetchData": {
        Type:     CronScheduler,  // 调度间隔设置为crontab表示式
        Spec:     "5 * * * *",  // 每小时开始的第五分钟执行fetch_db_data任务
        TaskFunc: TaskFetchDBData,
        TaskSignature: &tasks.Signature{
            Name: "fetch_db_data",
            Args: []tasks.Arg{
                {
                    Name:  "db_host",
                    Type:  "string",
                    Value: "schema://host:port",
                },
                {
                    Name:  "data_id",
                    Type:  "int64",
                    Value: int64(99999),
                },
            },
        },
    },
}
if err := CronJob.RegisterTasks(cronjobs); err != nil {
    panic(err)
}

// 开始运行定时任务，注意这个函数是阻塞的，在command之外的地方使用要格外小心
CronJob.StartCronJob()
```

## 注意事项

如果负责执行定时任务的 worker 存在任务堆积会比较繁忙，则定时任务的执行开始时间会存在一定的延迟。解决办法：

1. 让执行定时任务的 worker 使用专用的只有定时任务的队列，这样可以在很大程度上排除其他任务干扰造成的延迟
2. 如果不能设置专用队列，且无法接受执行延迟，可以使用 k8s 的 cronjob，但要注意 k8s 的 cronjob 只支持 crontab 表达式。

worker 需要自己处理任务超时和防止任务重叠运行，因为 machinery 的机制导致这些功能在 worker 上处理会比较方便，CronJobExt暂不支持。
