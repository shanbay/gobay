package custom_err

import (
	"github.com/getsentry/sentry-go"
)

func ExampleSentryExt_CaptureComplexError() {
	err := &CustomComplexError{
		Message:  "This is a complex error",
		MoreData: map[string]string{"key1": "val1", "key2": "val2"},
	}
	sentry.CaptureException(err)
}
