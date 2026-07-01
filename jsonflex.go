// Package jsonflex provides transparent, bidirectional conversion of JSON
// object keys between camelCase and snake_case.
//
// The typical use is as net/http middleware: incoming request bodies are
// converted from camelCase (as a JavaScript/TypeScript frontend sends them)
// to snake_case (as your Go structs and database expect), and outgoing
// response bodies are converted back from snake_case to camelCase. This lets
// you keep idiomatic Go structs on the server and idiomatic camelCase on the
// client without writing duplicate DTOs or per-field json tags.
//
// Conversion is recursive: nested objects and arrays are handled to any depth.
// Values are never interpreted or reformatted beyond what JSON re-encoding
// requires; only object keys are transformed.
//
// # Round-tripping and acronyms
//
// CamelToSnake and SnakeToCamel are not perfect inverses for keys containing
// acronyms. For example CamelToSnake("userID") == "user_id", and
// SnakeToCamel("user_id") == "userId" (not "userID"). This is intentional and
// matches what a JavaScript client conventionally uses. If you need exact
// preservation of specific keys, use the Exclude option on the middleware.
package jsonflex

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode"
)

// KeyFunc transforms a single JSON object key. CamelToSnake and SnakeToCamel
// are the built-in implementations; you may supply your own for custom naming
// conventions.
type KeyFunc func(string) string

// CamelToSnake converts a camelCase or PascalCase identifier to snake_case.
//
//	CamelToSnake("userName") == "user_name"
//	CamelToSnake("userID")   == "user_id"
//	CamelToSnake("APIKey")   == "api_key"
//	CamelToSnake("isHTTPS")  == "is_https"
//
// It is a no-op on strings that contain no uppercase letters, so calling it on
// an already-snake_case key is safe and idempotent.
func CamelToSnake(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				// Insert a separator at a lower/digit -> upper boundary
				// (fooBar), or at the end of an acronym run that is followed
				// by a lowercase letter (APIKey -> api_key).
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					b.WriteByte('_')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SnakeToCamel converts a snake_case identifier to camelCase.
//
//	SnakeToCamel("user_name") == "userName"
//	SnakeToCamel("user_id")   == "userId"
//	SnakeToCamel("_id")       == "_id"   (leading underscores are preserved)
//
// It is a no-op on strings that contain no underscores, so calling it on an
// already-camelCase key is safe and idempotent.
func SnakeToCamel(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	upperNext := false
	for _, r := range s {
		if r == '_' {
			// Preserve leading underscores (e.g. Mongo's "_id"); otherwise an
			// underscore signals that the next letter should be uppercased.
			if b.Len() == 0 {
				b.WriteRune('_')
				continue
			}
			upperNext = true
			continue
		}
		if upperNext {
			b.WriteRune(unicode.ToUpper(r))
			upperNext = false
			continue
		}
		b.WriteRune(r)
	}
	// A trailing underscore has no following letter to fold into; emit it so we
	// don't silently drop characters.
	if upperNext {
		b.WriteRune('_')
	}
	return b.String()
}

// ToSnakeCase converts every object key in a JSON document from camelCase to
// snake_case, recursively. Arrays and nested objects are handled. The input
// must be a valid JSON value; otherwise an error is returned and the caller
// should use the original bytes.
func ToSnakeCase(data []byte) ([]byte, error) {
	return Transform(data, CamelToSnake)
}

// ToCamelCase converts every object key in a JSON document from snake_case to
// camelCase, recursively.
func ToCamelCase(data []byte) ([]byte, error) {
	return Transform(data, SnakeToCamel)
}

// Transform decodes a JSON document, applies keyFn to every object key at every
// level of nesting, and re-encodes it. Values are preserved: numbers keep their
// original precision (no float64 rounding), and HTML characters (<, >, &) are
// not escaped.
//
// If data is not valid JSON, Transform returns an error and the caller should
// fall back to the original bytes rather than failing the request.
func Transform(data []byte, keyFn KeyFunc) ([]byte, error) {
	return transform(data, keyFn, nil)
}

// transform is the internal workhorse shared by the public API and the
// middleware. exclude names keys whose name is left unchanged and whose value
// is passed through untouched (not recursed into).
func transform(data []byte, keyFn KeyFunc, exclude map[string]struct{}) ([]byte, error) {
	v, err := unmarshal(data)
	if err != nil {
		return nil, err
	}
	converted := convertKeys(v, keyFn, exclude)
	return marshal(converted)
}

// convertKeys walks a decoded JSON value, rewriting object keys with keyFn.
// Excluded keys (and their entire subtree) are left as-is.
func convertKeys(v any, keyFn KeyFunc, exclude map[string]struct{}) any {
	switch node := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(node))
		for k, child := range node {
			if _, skip := exclude[k]; skip {
				out[k] = child
				continue
			}
			out[keyFn(k)] = convertKeys(child, keyFn, exclude)
		}
		return out
	case []any:
		for i, child := range node {
			node[i] = convertKeys(child, keyFn, exclude)
		}
		return node
	default:
		return v
	}
}

// unmarshal decodes JSON into a generic value tree, using json.Number so that
// large integers and high-precision numbers are not lossily converted to
// float64 and back.
func unmarshal(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// marshal re-encodes a value without HTML escaping and without the trailing
// newline that json.Encoder appends.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	return out, nil
}
