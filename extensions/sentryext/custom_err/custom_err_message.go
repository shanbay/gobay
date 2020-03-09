package custom_err

import (
	"github.com/getsentry/sentry-go"
)

func init() {
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.AddEventProcessor(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if ex, ok := hint.OriginalException.(*CustomComplexError); ok {
				for key, val := range ex.GimmeMoreData() {
					event.Extra[key] = val
				}
			}
			return event
		})
	})
}

type CustomComplexError struct {
	Message  string
	MoreData map[string]string
}

func (e *CustomComplexError) Error() string {
	return "CustomComplexError: " + e.Message
}
func (e *CustomComplexError) GimmeMoreData() map[string]string {
	return e.MoreData
}
