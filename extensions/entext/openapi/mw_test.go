package entopenapimw

import (
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/go-sql-driver/mysql"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/entext"
	"github.com/shanbay/gobay/testdata/ent"
)

var (
	entclient *ent.Client
	ext       *entext.EntExt
)

func init() {
	exts := map[gobay.Key]gobay.Extension{
		"entext": &entext.EntExt{
			NS: "db_",
			NewClient: func(drvopt interface{}) entext.Client {
				return ent.NewClient(drvopt.(ent.Option))
			},
			Driver: func(drv dialect.Driver) interface{} {
				return ent.Driver(drv)
			},
			IsNotFound:          ent.IsNotFound,
			IsConstraintFailure: ent.IsConstraintError,
			IsNotSingular:       ent.IsNotSingular,
		},
	}
	app, err := gobay.CreateApp("../../../testdata", "testing", exts)
	ext = app.Get("entext").(*entext.EntExt)
	if err != nil {
		panic(err)
	}
	entclient = app.Get("entext").Object().(*ent.Client)

	if err := app.Init(); err != nil {
		log.Panic(err)
	}
}

func TestOpenAPIRegularPanicWithoutMw(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("there is a panic")
		} else {
			panic("there should be a panic")
		}
	}()

	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("an error should be caught")
	})

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

func TestOpenAPIWithMwAndRegularPanic(t *testing.T) {
	// init sentry
	middleware := GetEntMw(ext)

	// handle
	defer func() {
		if err := recover(); err != "an error should be caught" {
			t.Errorf("Fail to catch error: %v", err)
		}
	}()

	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("an error should be caught")
	})
	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	newHandler := middleware(handler)
	newHandler.ServeHTTP(w, req)
}

func TestOpenAPIEntPanicWithoutMw(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("there is a panic")
			entErr, ok := err.(error)
			if !ok {
				t.Errorf("error should be ent error")
			}

			if ext.IsNotFound != nil && err != nil && ext.IsNotFound(entErr) {
				return
			} else {
				t.Errorf("error should be ent not found error")
			}

		} else {
			t.Errorf("there should be a panic")
		}
	}()

	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&ent.NotFoundError{})
	})

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

func TestOpenAPIWithMwAndEntNotFoundPanic(t *testing.T) {
	// init sentry
	middleware := GetEntMw(ext)

	// handle
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("Should not catch error: %v", err)
		}
	}()

	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&ent.NotFoundError{})
	})
	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	newHandler := middleware(handler)
	newHandler.ServeHTTP(w, req)
}

func TestOpenAPIWithMwAndEntAlreadyExistsPanic(t *testing.T) {
	// init sentry
	middleware := GetEntMw(ext)

	// handle
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("Should not catch error: %v", err)
		}
	}()

	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&ent.ConstraintError{})
	})
	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	newHandler := middleware(handler)
	newHandler.ServeHTTP(w, req)
}
