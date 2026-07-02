package jsonflex

import "bytes"

// Converter applies key-name conversion to JSON bodies according to a fixed set
// of options. It is the reusable engine shared by the built-in net/http
// Middleware and by the framework adapters (Gin, Echo, Chi) under ./adapters.
//
// A Converter is safe for concurrent use: it is configured once by New and only
// read thereafter, so a single instance can serve every request.
//
// Adapters typically call New once at startup, then use ConvertRequestBody and
// ConvertResponseBody inside the framework's request lifecycle, gating on
// IsJSONContentType and the RequestConversionEnabled / ResponseConversionEnabled
// flags.
type Converter struct {
	cfg *config
}

// New builds a Converter from the given options. The zero-option form
// (jsonflex.New()) converts request bodies camelCase -> snake_case and response
// bodies snake_case -> camelCase, matching Middleware's defaults.
func New(opts ...Option) *Converter {
	return &Converter{cfg: newConfig(opts)}
}

// RequestConversionEnabled reports whether request bodies should be converted.
func (c *Converter) RequestConversionEnabled() bool { return c.cfg.convertRequest }

// ResponseConversionEnabled reports whether response bodies should be converted.
func (c *Converter) ResponseConversionEnabled() bool { return c.cfg.convertResponse }

// ConvertRequestBody converts a JSON request body using the request key
// function (CamelToSnake by default). The input is returned unchanged if
// request conversion is disabled, the body is empty or whitespace, or the body
// is not valid JSON — so callers can apply it unconditionally without risking a
// failed request on malformed input.
func (c *Converter) ConvertRequestBody(body []byte) []byte {
	return c.convert(body, c.cfg.convertRequest, c.cfg.requestKeyFn, c.cfg.requestStyle)
}

// ConvertResponseBody converts a JSON response body using the response key
// function (SnakeToCamel by default). Like ConvertRequestBody, it passes
// disabled, empty, and invalid input through untouched.
func (c *Converter) ConvertResponseBody(body []byte) []byte {
	return c.convert(body, c.cfg.convertResponse, c.cfg.responseKeyFn, c.cfg.responseStyle)
}

func (c *Converter) convert(body []byte, enabled bool, fn KeyFunc, style keyStyle) []byte {
	if !enabled || len(bytes.TrimSpace(body)) == 0 {
		return body
	}
	if out, err := convertBody(body, fn, style, c.cfg.exclude); err == nil {
		return out
	}
	// Invalid JSON: leave the caller's bytes untouched.
	return body
}

// convertBody dispatches to the byte-level engine for the built-in directions
// and to the tree-walk engine for custom KeyFuncs.
func convertBody(data []byte, fn KeyFunc, style keyStyle, exclude map[string]struct{}) ([]byte, error) {
	if style == styleCustom {
		return transform(data, fn, exclude)
	}
	return transformBytes(data, style, exclude)
}
