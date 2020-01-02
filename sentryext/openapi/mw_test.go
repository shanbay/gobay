package sentryopenapimw

import (
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/sentryext"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

var sentry_extension sentryext.SentryExt

var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	panic("an error should be caught by sentry_mw")
})

func init() {
	sentry_extension = sentryext.SentryExt{NS: "sentry_"}

	app, err := gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"sentry": &sentry_extension,
		},
	)
	if err != nil {
		log.Panic(err)
	}
	if err := app.Init(); err != nil {
		log.Panic(err)
	}
}

func TestOpenAPIWithoutSentry(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("there is a panic")
		} else {
			panic("there should be a panic")
		}
	}()

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

func TestOpenAPIWithSentry(t *testing.T) {
	// init sentry
	middleware, _ := GetMiddleWare(&sentry_extension)

	// handle
	defer func() {
		if err := recover(); err != "an error should be caught by sentry_mw" {
			log.Fatal("Fail: ", err)
		}
	}()
	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	new_handler := middleware(handler)
	new_handler.ServeHTTP(w, req)
}
