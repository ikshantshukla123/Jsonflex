package jsonflex

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// config holds the resolved middleware settings. It is built from the default
// values and any Option functions passed to Middleware.
type config struct {
	convertRequest  bool
	convertResponse bool
	requestKeyFn    KeyFunc
	responseKeyFn   KeyFunc
	exclude         map[string]struct{}
}

// Option configures the middleware. See With* and Exclude.
type Option func(*config)

// WithRequestConversion enables or disables conversion of incoming request
// bodies (camelCase -> snake_case by default). Enabled by default.
func WithRequestConversion(enabled bool) Option {
	return func(c *config) { c.convertRequest = enabled }
}

// WithResponseConversion enables or disables conversion of outgoing response
// bodies (snake_case -> camelCase by default). Enabled by default.
//
// Note: enabling response conversion buffers the full response body so it can
// be re-encoded. Disable it for streaming endpoints (e.g. Server-Sent Events).
func WithResponseConversion(enabled bool) Option {
	return func(c *config) { c.convertResponse = enabled }
}

// WithRequestKeyFunc overrides the key transform applied to request bodies.
// The default is CamelToSnake.
func WithRequestKeyFunc(fn KeyFunc) Option {
	return func(c *config) {
		if fn != nil {
			c.requestKeyFn = fn
		}
	}
}

// WithResponseKeyFunc overrides the key transform applied to response bodies.
// The default is SnakeToCamel.
func WithResponseKeyFunc(fn KeyFunc) Option {
	return func(c *config) {
		if fn != nil {
			c.responseKeyFn = fn
		}
	}
}

// Exclude marks keys that must not be renamed. An excluded key keeps its
// original name and its entire value is passed through untouched (the
// converter does not recurse into it). This is useful for fields that hold
// arbitrary, caller-controlled JSON whose shape you do not want to alter.
//
// Exclusion is matched against the key as it appears in the document being
// processed, so list request keys in their incoming (camelCase) form and
// response keys in their outgoing (snake_case) form as appropriate.
func Exclude(keys ...string) Option {
	return func(c *config) {
		if c.exclude == nil {
			c.exclude = make(map[string]struct{}, len(keys))
		}
		for _, k := range keys {
			c.exclude[k] = struct{}{}
		}
	}
}

func newConfig(opts []Option) *config {
	c := &config{
		convertRequest:  true,
		convertResponse: true,
		requestKeyFn:    CamelToSnake,
		responseKeyFn:   SnakeToCamel,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Middleware returns net/http middleware that transparently converts JSON key
// naming on the way in and out of your handlers.
//
//	mux := http.NewServeMux()
//	mux.Handle("/api/", apiHandler)
//	http.ListenAndServe(":8080", jsonflex.Middleware()(mux))
//
// Only requests and responses whose Content-Type is application/json are
// touched. Bodies that are empty or not valid JSON are passed through
// unchanged rather than causing an error, so the middleware is safe to apply
// broadly.
func Middleware(opts ...Option) func(http.Handler) http.Handler {
	cfg := newConfig(opts)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.convertRequest {
				cfg.convertRequestBody(r)
			}

			if !cfg.convertResponse {
				next.ServeHTTP(w, r)
				return
			}

			rec := &responseRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			cfg.writeResponse(w, rec)
		})
	}
}

// convertRequestBody rewrites r.Body in place when it carries a JSON payload.
// On any read or parse problem the original body is restored so the downstream
// handler sees exactly what the client sent.
func (c *config) convertRequestBody(r *http.Request) {
	if r.Body == nil || !isJSONContentType(r.Header.Get("Content-Type")) {
		return
	}

	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		// Give the handler a body it can still attempt to read; the read error
		// will resurface there.
		r.Body = io.NopCloser(bytes.NewReader(body))
		return
	}

	if len(bytes.TrimSpace(body)) > 0 {
		if converted, cerr := transform(body, c.requestKeyFn, c.exclude); cerr == nil {
			body = converted
		}
		// On a parse error we intentionally keep the original body.
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

// writeResponse flushes a buffered response, converting the body first when it
// is JSON.
func (c *config) writeResponse(w http.ResponseWriter, rec *responseRecorder) {
	body := rec.buf.Bytes()

	if isJSONContentType(rec.Header().Get("Content-Type")) && len(bytes.TrimSpace(body)) > 0 {
		if converted, err := transform(body, c.responseKeyFn, c.exclude); err == nil {
			body = converted
		}
	}

	status := rec.status
	if status == 0 {
		status = http.StatusOK
	}

	// The length changed, so any Content-Length the handler set is stale.
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	w.Write(body)
}

// responseRecorder buffers a handler's response so the body can be re-encoded
// before it is written to the real ResponseWriter. Header mutations pass
// straight through to the wrapped writer's header map.
type responseRecorder struct {
	http.ResponseWriter
	buf    bytes.Buffer
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	// Capture the status but defer the actual write until the body is ready.
	r.status = status
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.buf.Write(b)
}

// isJSONContentType reports whether a Content-Type header denotes JSON,
// tolerating parameters such as "application/json; charset=utf-8" and
// "+json" suffixes like application/vnd.api+json.
func isJSONContentType(ct string) bool {
	if ct == "" {
		return false
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}
