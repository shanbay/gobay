package entopenapimw

import (
	"encoding/json"
	"net/http"

	"github.com/go-openapi/runtime/middleware"
	"github.com/shanbay/gobay/extensions/entext"
)

func respondHTTPError(w http.ResponseWriter, statusCode int, defaultMessage string, entErr error) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	msg, jsonErr := json.Marshal(map[string]string{
		"msg":    defaultMessage,
		"detail": entErr.Error(),
	})
	if jsonErr != nil {
		msg = []byte("{\"msg\":\"" + defaultMessage + "\"}")
	}
	_, _ = w.Write(msg)
}

// GetEntMw - Get ent middleware
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
						respondHTTPError(w, 404, "Not Found Error", entErr)
						return
					} else if e.IsConstraintFailure != nil && err != nil && e.IsConstraintFailure(entErr) {
						respondHTTPError(w, 400, "Already Exists Error", entErr)
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
