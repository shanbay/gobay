package openapi

import (
	"net/http"

	"github.com/go-openapi/runtime/middleware"
)

// ChainMiddlewares wraps http handlers and return a handler that exec passin http handlers in sequence
// Notice: income handler sequence(from left to right) stands for from outer to inner
// say: builders = b1, b2, b3, return b1(b2(b3))
func ChainMiddlewares(builders ...middleware.Builder) middleware.Builder {
	n := len(builders)
	if n == 0 {
		return middleware.PassthroughBuilder
	}

	return func(h http.Handler) http.Handler {
		chainer := func(outer middleware.Builder, inner http.Handler) http.Handler {
			return outer(inner)
		}

		chainedHandler := h
		for i := n - 1; i >= 0; i-- {
			chainedHandler = chainer(builders[i], chainedHandler)
		}

		return chainedHandler
	}

}
