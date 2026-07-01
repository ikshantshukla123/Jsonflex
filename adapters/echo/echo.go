// Package echoflex adapts jsonflex to the Echo web framework.
//
//	e := echo.New()
//	e.Use(echoflex.Middleware())
//
// It accepts the same jsonflex.Option values as the core middleware:
//
//	e.Use(echoflex.Middleware(jsonflex.Exclude("rawMeta")))
package echoflex

import (
	"github.com/ikshantshukla123/jsonflex"
	"github.com/labstack/echo/v4"
)

// Middleware returns Echo middleware that converts request bodies
// camelCase -> snake_case and response bodies snake_case -> camelCase.
//
// It reuses the core net/http middleware through echo.WrapMiddleware, so its
// behaviour is identical: only application/json (and +json) bodies are touched,
// empty or invalid JSON passes through unchanged, and response conversion
// buffers the body (disable it with jsonflex.WithResponseConversion(false) for
// streaming endpoints).
func Middleware(opts ...jsonflex.Option) echo.MiddlewareFunc {
	return echo.WrapMiddleware(jsonflex.Middleware(opts...))
}
