package cronjobext

import (
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/shanbay/gobay"
)

var (
	cronjobOne CronJobExt
	cronjobTwo CronJobExt
)

func init() {
	cronjobOne = CronJobExt{NS: "one_cronjob_"}
	cronjobTwo = CronJobExt{NS: "two_cronjob_"}

	app, _ := gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"onecronjob": &cronjobOne,
			"twocronjob": &cronjobTwo,
		},
	)
	if err := app.Init(); err != nil {
		log.Panic(err)
	}
}

func TestCronJobExtTimeZone(t *testing.T) {
	if cronjobOne.TimeZone.String() != "Asia/Shanghai" {
		t.Errorf("timezone error: want: %v got: %v", "Asia/Shanghai", cronjobOne.TimeZone.String())
	}

	if cronjobTwo.TimeZone.String() != "UTC" {
		t.Errorf("timezone error: want: %v got: %v", "UTC", cronjobTwo.TimeZone.String())
	}
}

func TestReuseOtherConfig(t *testing.T) {
	if cronjobOne.config.DefaultQueue != "gobay.task.one" {
		t.Errorf("reuse error: want: %v got: %v", "gobay.task.one", cronjobOne.config.DefaultQueue)
	}

	if cronjobTwo.config.ReuseOther != "two_asynctask_" {
		t.Errorf("reuse error: want: %v got: %v", "two_asynctask_", cronjobTwo.config.ReuseOther)
	}
	if cronjobTwo.config.DefaultQueue != "gobay.task.two" {
		t.Errorf("reuse error: want: %v got: %v", "gobay.task.two", cronjobTwo.config.DefaultQueue)
	}
}

func TaskAdd(args ...int64) (int64, error) {
	sub := int64(0)
	for _, arg := range args {
		sub += arg
	}
	return sub, nil
}

func TaskSub(arg1, arg2 int64) (int64, error) {
	return arg1 - arg2, nil
}

func TestRegisterCronjobTask(t *testing.T) {
	testData := []struct {
		tasks     map[string]*CronJobTask
		wantError bool
		jobNum    int
	}{
		{
			tasks: map[string]*CronJobTask{
				"add": {
					Type:     DurationScheduler,
					Spec:     "1s",
					TaskFunc: TaskAdd,
					TaskSignature: &tasks.Signature{
						Name: "add_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
				"sub": {
					Type:     DurationScheduler,
					Spec:     "1m",
					TaskFunc: TaskSub,
					TaskSignature: &tasks.Signature{
						Name: "sub_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
			},
			wantError: false,
			jobNum:    2,
		},
		{
			tasks: map[string]*CronJobTask{
				"sub": {
					Type:     CronScheduler,
					Spec:     "5 4 * * *",
					TaskFunc: TaskSub,
					TaskSignature: &tasks.Signature{
						Name: "sub_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
			},
			wantError: false,
			jobNum:    1,
		},
		{
			tasks: map[string]*CronJobTask{
				"sub": {
					Type:     CronScheduler,
					Spec:     "5 - * * *",
					TaskFunc: TaskSub,
					TaskSignature: &tasks.Signature{
						Name: "sub_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
			},
			wantError: true,
			jobNum:    0,
		},
		{
			tasks: map[string]*CronJobTask{
				"sub": {
					Type:     DurationScheduler,
					Spec:     "error",
					TaskFunc: TaskSub,
					TaskSignature: &tasks.Signature{
						Name: "sub_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
			},
			wantError: true,
			jobNum:    0,
		},
		{
			tasks: map[string]*CronJobTask{
				"sub1": {
					Type:     DurationScheduler,
					Spec:     "1s",
					TaskFunc: TaskSub,
					TaskSignature: &tasks.Signature{
						Name: "sub_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
				"sub2": {
					Type:     DurationScheduler,
					Spec:     "error",
					TaskFunc: TaskSub,
					TaskSignature: &tasks.Signature{
						Name: "sub_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
			},
			wantError: true,
			jobNum:    0,
		},
		{
			tasks: map[string]*CronJobTask{
				"add1": {
					Type:     DurationScheduler,
					Spec:     "1h",
					TaskFunc: TaskAdd,
					TaskSignature: &tasks.Signature{
						Name: "add_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
				"add2": {
					Type:     DurationScheduler,
					Spec:     "2m",
					TaskFunc: TaskAdd,
					TaskSignature: &tasks.Signature{
						Name: "add_cron",
						Args: []tasks.Arg{
							{
								Name:  "a1",
								Type:  "int64",
								Value: int64(1234),
							},
							{
								Name:  "a2",
								Type:  "int64",
								Value: int64(4321),
							},
						},
					},
				},
			},
			wantError: false,
			jobNum:    2,
		},
	}
	for _, td := range testData {
		if err := cronjobOne.RegisterTasks(td.tasks); (err != nil) != td.wantError {
			if td.wantError {
				t.Error("want error but got nil")
			} else {
				t.Error(err)
			}
		}
		if cronjobOne.scheduler.Len() != td.jobNum {
			t.Errorf("length error want: %v got: %v", td.jobNum, cronjobOne.scheduler.Len())
		}
		cronjobOne.RemoveAllJobs()
	}
}

func TestHealthCheck(t *testing.T) {
	cronjobs := map[string]*CronJobTask{
		"sub": {
			Type:     DurationScheduler,
			Spec:     "1s",
			TaskFunc: TaskSub,
			TaskSignature: &tasks.Signature{
				Name: "sub_cron",
				Args: []tasks.Arg{
					{
						Name:  "a1",
						Type:  "int64",
						Value: int64(1234),
					},
					{
						Name:  "a2",
						Type:  "int64",
						Value: int64(4321),
					},
				},
			},
		},
	}
	if err := cronjobOne.RegisterTasks(cronjobs); err != nil {
		t.Error(err)
	}
	go cronjobOne.StartCronJob(true)
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://127.0.0.1:5001/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("%v %s", resp, err)
	}
	cronjobOne.RemoveAllJobs()
}
