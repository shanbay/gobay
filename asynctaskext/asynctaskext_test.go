package asynctaskext

import (
	"log"
	"testing"
	"time"

	"github.com/RichardKnop/machinery/v1/tasks"

	"github.com/shanbay/gobay"
)

var (
	app  *gobay.Application
	task AsyncTaskExt
)

func init() {
	task = AsyncTaskExt{NS: "asynctask_"}

	app, _ := gobay.CreateApp(
		"../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"asynctask": &task,
		},
	)
	if err := app.Init(); err != nil {
		log.Panic(err)
	}
}

func TestPushConsume(t *testing.T) {
	if err := task.RegisterWorkerHandler("add", TaskAdd); err != nil {
		t.Error(err)
	}
	go func() {
		if err := task.StartWorker("gobay.task"); err != nil {
			t.Error(err)
		}
	}()

	sign := &tasks.Signature{
		Name: "add",
		Args: []tasks.Arg{
			{
				Type: "int64",
				Value: 1,
			},
			{
				Type: "int64",
				Value: 2,
			},
			{
				Type: "int64",
				Value: 3,
			},
		},
	}
	if asyncResult, err := task.SendTask(sign); err != nil {
		t.Error(err)
	} else if results, err := asyncResult.Get(time.Millisecond * 5); err != nil {
		t.Error(err)
	} else if res, ok := results[0].Interface().(int64); !ok || res != 6 {
		t.Error("result error")
	}

}
func TaskAdd(args ...int64) (int64, error) {
	sum := int64(0)
	for _, arg := range args {
		sum += arg
	}
	return sum, nil
}
