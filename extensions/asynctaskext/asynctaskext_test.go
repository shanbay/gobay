package asynctaskext

import (
	"context"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/RichardKnop/machinery/v1/backends/result"
	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/stretchr/testify/assert"

	"github.com/shanbay/gobay"
)

var (
	taskOne   AsyncTaskExt
	taskTwo   AsyncTaskExt
	taskThree AsyncTaskExt
)

func init() {
	taskOne = AsyncTaskExt{NS: "one_asynctask_"}
	taskTwo = AsyncTaskExt{NS: "two_asynctask_"}
	taskThree = AsyncTaskExt{NS: "two_asynctask_"}

	app, _ := gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"oneasynctask":   &taskOne,
			"twoasynctask":   &taskTwo,
			"threeasynctask": &taskThree,
		},
	)
	if err := app.Init(); err != nil {
		log.Panic(err)
	}
}

func TaskAdd(args ...int64) (int64, error) {
	sum := int64(0)
	for _, arg := range args {
		sum += arg
	}
	return sum, nil
}

func TaskSub(arg1, arg2 int64) (int64, error) {
	return arg1 - arg2, nil
}

func TaskSubWithContext(ctx context.Context, arg1, arg2 int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return arg1 - arg2, nil
}

func TestPushConsume(t *testing.T) {
	if err := taskOne.RegisterWorkerHandlers(map[string]interface{}{
		"add": TaskAdd, "sub": TaskSub, "subCtx": TaskSubWithContext,
	}); err != nil {
		t.Error(err)
	}
	go func() {
		// use default queue
		if err := taskOne.StartWorker("", 1, true); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(500 * time.Millisecond) // Make sure the worker is started
	go func() {
		if err := taskOne.StartWorker("gobay.task_sub", 1, true); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(500 * time.Millisecond) // Make sure the workers is started

	// health check
	resp, err := http.Get("http://127.0.0.1:5000/health?timeout=5&queue=gobay.task_sub")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("%v %s", resp, err)
	}
	resp, err = http.Get("http://127.0.0.1:5000/health?timeout=5&queue=")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("%v %s", resp, err)
	}
	resp, err = http.Get("http://127.0.0.1:5000/health?timeout=5")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("%v %s", resp, err)
	}
	resp, err = http.Get("http://127.0.0.1:5000/health?timeout=5&queue=nosuchqueue")
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Errorf("%v %s", resp, err)
	}

	signs := []*tasks.Signature{
		{
			Name: "add",
			Args: []tasks.Arg{ // use default queue
				{
					Type:  "int64",
					Value: 1,
				},
				{
					Type:  "int64",
					Value: 2,
				},
				{
					Type:  "int64",
					Value: 3,
				},
			},
		},
		{
			Name:       "sub",
			RoutingKey: "gobay.task_sub",
			Args: []tasks.Arg{
				{
					Type:  "int64",
					Value: 7,
				},
				{
					Type:  "int64",
					Value: 1,
				},
			},
		},
		{
			Name:       "subCtx",
			RoutingKey: "gobay.task_sub",
			Args: []tasks.Arg{
				{
					Type:  "int64",
					Value: 10,
				},
				{
					Type:  "int64",
					Value: 4,
				},
			},
		},
	}
	for _, sign := range signs {
		var (
			asyncResult *result.AsyncResult
			err         error
		)
		if sign.Name == "subCtx" {
			asyncResult, err = taskOne.SendTaskWithContext(context.Background(), sign)
		} else {
			asyncResult, err = taskOne.SendTask(sign)
		}
		if err != nil {
			t.Error(err)
		} else if results, err := asyncResult.Get(time.Millisecond * 5); err != nil {
			t.Error(err)
		} else if res, ok := results[0].Interface().(int64); !ok || res != 6 {
			t.Error("result error")
		}
	}
}

func TestMultiTaskExtStartWorker(t *testing.T) {
	t.Run("1: 第一个 task StartWorker, 允许 healthcheck, 正常", func(t *testing.T) {
		go func() {
			// use default queue
			if err := taskOne.StartWorker("", 1, true); err != nil {
				t.Error(err)
			}
		}()
	})

	t.Run("2: 第二个 task StartWorker, 不允许 healthcheck, 正常运行", func(t *testing.T) {
		go func() {
			if err := taskTwo.StartWorker("", 1, false); err != nil {
				t.Error(err)
			}
		}()
	})

	t.Run("3: 第二个 task StartWorker, 允许 healthcheck, 会 panic", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = taskTwo.StartWorker("", 1, true)
		})
	})
}

func TestStartWorkerLimitConcurrency(t *testing.T) {
	t.Run("concurrency 1", func(t *testing.T) {
		go func() {
			if err := taskThree.StartWorker("", 1, false); err != nil {
				t.Error(err)
			}
		}()
	})
	t.Run("concurrency 1000", func(t *testing.T) {
		go func() {
			if err := taskThree.StartWorker("", 1000, false); err != nil {
				t.Error(err)
			}
		}()
	})
	t.Run("concurrency 20", func(t *testing.T) {
		go func() {
			if err := taskThree.StartWorker("", 20, false); err != nil {
				t.Error(err)
			}
		}()
	})

	time.Sleep(5 * time.Second) // wait for all workers got to running
	assert.Len(t, taskThree.workers, 3)
	for _, w := range taskThree.workers {
		assert.EqualValues(t, concurrency_limit, w.Concurrency)
	}
}
