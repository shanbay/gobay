package entopenapimw

import (
	"encoding/json"
	"net/http"

	"github.com/go-openapi/runtime/middleware"
	"github.com/shanbay/gobay/extensions/entext"
)

func GetEntMw(e *entext.EntExt) middleware.Builder {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			defer func() {
				if err := recover(); err != nil {
					entErr, ok := err.(error)
					if !ok {
						panic(err)
					}

					if e.IsNotFound != nil && err != nil && e.IsNotFound(entErr) {
						h := w.Header()
						h.Set("Content-Type", "application/json; charset=utf-8")
						w.WriteHeader(404)

						msg, jsonErr := json.Marshal(map[string]string{
							"msg":    "Not Found Error",
							"detail": entErr.Error(),
						})
						if jsonErr != nil {
							msg = []byte("{\"msg\":\"Not Found Error\"}"}
						}
						w.Write(msg)
						return
					} else if e.IsConstraintFailure != nil && err != nil && e.IsConstraintFailure(entErr) {
						h := w.Header()
						h.Set("Content-Type", "application/json; charset=utf-8")
						w.WriteHeader(400)
						msg, jsonErr := json.Marshal(map[string]string{
							"msg":    "Already Exists Error",
							"detail": entErr.Error(),
						})
						if jsonErr != nil {
							msg = []byte("{\"msg\":\"Already Exists Error\"}"}
						}
						w.Write(msg)
						return
					} else {
						panic(err)
					}
				}
			}()

			handler.ServeHTTP(w, req)
		})
	}
}
