# 定时任务 cronjob

其实没有什么特殊的设定，我们会使用 K8S 的 cronjob 来完成定时任务的功能。

项目中，只需要创建一个跟 httpsvc/grpcsvc/asynctask 一样的一个执行 script 即可。

## 创建 cronjob 的 command

```go
package actions

import (
  "context"
  "log"

  "github.com/shanbay/gobay"
  "github.com/spf13/cobra"
)

func SomeCronJob(cmd *cobra.Command, args []string) {
  // 
}

func init() {
  cmd := &cobra.Command{
    Use: "cron_job_command_name",
    Run: SomeCronJob,
  }
  rootCmd.AddCommand(cmd)
}
```

## 定时任务运行这个 command

```sh
make run COMMAND=cron_job_command_name
```

或者 build 成可执行文件并直接执行

```sh
make build_release
<project_dir>/app/.dist/app --env <env_vars> cron_job_command_name
```
