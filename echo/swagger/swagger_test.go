package swagger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func Test_swagger_doc(t *testing.T) {
	req, _ := http.NewRequest("GET", "/gordon/apidocs", nil)

	// 200
	mw := SwaggerDoc("/gordon", []byte{})
	e := echo.New()
	e.Pre(mw)
	handler := e.Server.Handler
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != 200 {
		t.Errorf("Wrong swagger resp code: %v, want: 200 ", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != "text/html; charset=UTF-8" {
		t.Errorf("Wrong response content type: %v", recorder.Header().Get("Content-Type"))
	}
	respString := recorder.Body.String()
	if !strings.Contains(respString, "<title>API documentation</title>") {
		t.Errorf("Wrong response bytes")
	}

	// authorize 403
	mw = SwaggerDoc("/gordon", []byte{}, SetSwaggerAuthorizer(func(req *http.Request) bool {
		return false
	}))
	e = echo.New()
	e.Pre(mw)
	handler = e.Server.Handler
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != 403 {
		t.Errorf("Wrong swagger resp code: %v, want: 403 ", recorder.Code)
	}

	// HTTPS
	req, _ = http.NewRequest("GET", "/gordon/swagger.json", nil)
	mw = SwaggerDoc("/gordon", []byte(`"schemes": [
    "http"
  ],`), SetSwaggerIsHTTPS(true))
	e = echo.New()
	e.Pre(mw)
	handler = e.Server.Handler
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != 200 {
		t.Errorf("Wrong swagger resp code: %v, want: 200 ", recorder.Code)
	}
	respString = recorder.Body.String()
	if !strings.Contains(respString, `"schemes": [
    "https"
  ],`) {
		t.Errorf("Wrong response %v", respString)
	}

	// OPTIONS with CORS mw
	// will return 204 with CORS headers, and the body should be empty
	req, _ = http.NewRequest("OPTIONS", "/gordon/apidocs", nil)
	req.Header.Set(echo.HeaderAccessControlRequestMethod, "GET")
	req.Header.Set(echo.HeaderOrigin, "https://example.com")
	mw = SwaggerDoc("/gordon", []byte{})
	e = echo.New()
	e.Pre(mw) // will be skipped and call `next(c)`
	e.Pre(middleware.CORS())
	handler = e.Server.Handler
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != 204 {
		t.Errorf("Wrong swagger resp code: %v, want: 204 ", recorder.Code)
	}
	if !strings.Contains(recorder.Header().Get("Access-Control-Allow-Origin"), "*") {
		t.Errorf("Wrong response %v", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if recorder.Body.String() != "" {
		t.Errorf("Wrong response %v", recorder.Body.String())
	}	
}
