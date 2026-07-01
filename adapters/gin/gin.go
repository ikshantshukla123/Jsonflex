// Package ginflex adapts jsonflex to the Gin web framework.
//
//	r := gin.New()
//	r.Use(ginflex.Middleware())
//
// It accepts the same jsonflex.Option values as the core middleware, so
// exclusions, direction toggles, and custom key functions all work:
//
//	r.Use(ginflex.Middleware(jsonflex.Exclude("rawMeta")))
package ginflex

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ikshantshukla123/jsonflex"
)

// Middleware returns Gin middleware that converts request bodies
// camelCase -> snake_case and response bodies snake_case -> camelCase. Only
// application/json (and +json) bodies are touched; empty or invalid JSON is
// passed through unchanged.
//
// Response conversion buffers the full response body so it can be re-encoded;
// disable it with jsonflex.WithResponseConversion(false) for streaming
// endpoints.
func Middleware(opts ...jsonflex.Option) gin.HandlerFunc {
	conv := jsonflex.New(opts...)
	return func(c *gin.Context) {
		if conv.RequestConversionEnabled() {
			convertRequest(conv, c.Request)
		}

		if !conv.ResponseConversionEnabled() {
			c.Next()
			return
		}

		bw := &bufferedWriter{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = bw
		c.Next()
		flush(conv, bw)
	}
}

// convertRequest rewrites the request body in place when it is JSON. A read
// error restores the original body so the handler can surface it.
func convertRequest(conv *jsonflex.Converter, req *http.Request) {
	if req == nil || req.Body == nil || !jsonflex.IsJSONContentType(req.Header.Get("Content-Type")) {
		return
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return
	}

	body = conv.ConvertRequestBody(body)
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

// flush converts the buffered body if it is JSON and writes it to the real
// Gin ResponseWriter.
func flush(conv *jsonflex.Converter, bw *bufferedWriter) {
	body := bw.buf.Bytes()
	if jsonflex.IsJSONContentType(bw.Header().Get("Content-Type")) {
		body = conv.ConvertResponseBody(body)
	}

	status := bw.status
	if status == 0 {
		status = http.StatusOK
	}

	real := bw.ResponseWriter
	real.Header().Set("Content-Length", strconv.Itoa(len(body)))
	real.WriteHeader(status)
	real.Write(body)
}

// bufferedWriter captures the handler's response so the body can be converted
// before it reaches the client. It embeds gin.ResponseWriter to satisfy the
// full interface and overrides only the write path. Header mutations pass
// straight through to the embedded writer's header map.
type bufferedWriter struct {
	gin.ResponseWriter
	buf    *bytes.Buffer
	status int
}

// WriteHeader records the status but does not flush it; flush writes it later.
func (w *bufferedWriter) WriteHeader(status int) { w.status = status }

// WriteHeaderNow is a no-op so Gin does not flush headers to the client before
// the buffered body has been converted.
func (w *bufferedWriter) WriteHeaderNow() {}

func (w *bufferedWriter) Write(b []byte) (int, error)       { return w.buf.Write(b) }
func (w *bufferedWriter) WriteString(s string) (int, error) { return w.buf.WriteString(s) }
func (w *bufferedWriter) Size() int                         { return w.buf.Len() }
func (w *bufferedWriter) Written() bool                     { return w.status != 0 || w.buf.Len() > 0 }

func (w *bufferedWriter) Status() int {
	if w.status == 0 {
		return w.ResponseWriter.Status()
	}
	return w.status
}
