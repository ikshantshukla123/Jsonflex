// Package chiflex adapts jsonflex to the Chi router.
//
// Chi middleware uses the standard func(http.Handler) http.Handler signature,
// which is exactly what jsonflex.Middleware already returns — so jsonflex works
// with Chi out of the box:
//
//	r := chi.NewRouter()
//	r.Use(jsonflex.Middleware())        // works directly
//	r.Use(chiflex.Middleware())         // identical, provided for symmetry
//
// This package exists for discoverability and API parity with the Gin and Echo
// adapters; Middleware is a thin pass-through to jsonflex.Middleware.
package chiflex

import (
	"net/http"

	"github.com/ikshantshukla123/jsonflex"
)

// Middleware returns Chi-compatible middleware that converts request bodies
// camelCase -> snake_case and response bodies snake_case -> camelCase. It is
// equivalent to calling jsonflex.Middleware directly.
func Middleware(opts ...jsonflex.Option) func(http.Handler) http.Handler {
	return jsonflex.Middleware(opts...)
}
