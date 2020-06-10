package custom_err

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/sentryext"
	"testing"
	"time"
)

func ExampleSentryExt_CaptureComplexError() {
	var err = &CustomComplexError{
		Message:  "This is a complex error",
		MoreData: map[string]string{"key1": "val1", "key2": "val2"},
	}
	sentry.CaptureException(err)
}

func fixtureApp() (*gobay.Application, error) {
	sentryext := &sentryext.SentryExt{NS: "sentry_"}
	exts := map[gobay.Key]gobay.Extension{
		"sentry": sentryext,
	}
	if ap, err := gobay.CreateApp("../../../testdata/", "testing", exts); err != nil {
		return nil, err
	} else {
		return ap, nil
	}
}

func Test_CaptureComplexError(t *testing.T) {
	defer func() {
		if e := recover(); e != nil {
			if stre, ok := e.(error); !ok || stre.Error() != "Known error" {
				t.Errorf("Error is: %s", e)
			}
		}
	}()
	if _, err := fixtureApp(); err != nil {
		t.Errorf(err.Error())
	}

	var err = &CustomComplexError{
		Message:  "This is a complex error",
		MoreData: map[string]string{"key1": "val1", "key2": "val2"},
	}
	// 允许捕获正常的错误
	sentry.CaptureException(err)
	if !sentry.Flush(5 * time.Second) {
		t.Errorf("Flush failed")
	}
	// 注入一个错误
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.AddEventProcessor(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			e := errors.New("Known error")
			panic(e)
		})
	})
	sentry.CaptureException(err)
	if !sentry.Flush(5 * time.Second) {
		t.Errorf("Flush failed")
	}
}
