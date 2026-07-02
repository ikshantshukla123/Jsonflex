package jsonflex

import (
	"encoding/json"
	"errors"
)

// This file implements the byte-level conversion engine (the "v4" fast path).
//
// Instead of decoding JSON into a map/slice tree and re-encoding it — which
// allocates a boxed value for every element — the scanner walks the raw bytes,
// copies every structural token, value, and string value verbatim, and rewrites
// only object keys. Because values are never decoded, integer precision and
// byte content are preserved exactly, and the common case (ASCII keys, no
// escapes) does no per-key heap allocation at all.
//
// The scanner validates as it goes and returns errInvalidJSON on malformed
// input, so the Converter's pass-through-on-error contract is preserved. It is
// used for the built-in camelCase<->snake_case directions; custom KeyFuncs
// still go through the tree-walk engine in jsonflex.go.

// keyStyle selects which conversion engine and direction a Converter uses.
type keyStyle int

const (
	styleCamelToSnake keyStyle = iota // default request direction
	styleSnakeToCamel                 // default response direction
	styleCustom                       // caller-supplied KeyFunc -> tree-walk engine
)

// maxDepth bounds nesting to avoid unbounded recursion (and stack exhaustion)
// on adversarial input. It matches the order of magnitude of encoding/json's
// own limit.
const maxDepth = 10000

var errInvalidJSON = errors.New("jsonflex: invalid JSON")

// transformBytes rewrites object keys in a JSON document at the byte level.
// style must be styleCamelToSnake or styleSnakeToCamel.
func transformBytes(data []byte, style keyStyle, exclude map[string]struct{}) ([]byte, error) {
	s := &scanner{
		src:     data,
		out:     make([]byte, 0, len(data)+len(data)/8+16),
		style:   style,
		exclude: exclude,
	}
	if err := s.value(0, true); err != nil {
		return nil, err
	}
	s.ws()
	if s.pos != len(s.src) {
		return nil, errInvalidJSON // trailing garbage
	}
	return s.out, nil
}

type scanner struct {
	src     []byte
	pos     int
	out     []byte
	style   keyStyle
	exclude map[string]struct{}
}

func (s *scanner) peek() byte {
	if s.pos < len(s.src) {
		return s.src[s.pos]
	}
	return 0
}

// ws copies any run of insignificant whitespace to the output verbatim.
func (s *scanner) ws() {
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case ' ', '\t', '\n', '\r':
			s.out = append(s.out, s.src[s.pos])
			s.pos++
		default:
			return
		}
	}
}

// value scans a single JSON value. conv reports whether object keys inside it
// should be converted; it is false inside an excluded subtree.
func (s *scanner) value(depth int, conv bool) error {
	s.ws()
	if s.pos >= len(s.src) {
		return errInvalidJSON
	}
	c := s.src[s.pos]
	switch {
	case c == '{':
		return s.object(depth, conv)
	case c == '[':
		return s.array(depth, conv)
	case c == '"':
		return s.copyString() // string values are always copied verbatim
	case c == '-' || (c >= '0' && c <= '9'):
		return s.number()
	case c == 't':
		return s.literal("true")
	case c == 'f':
		return s.literal("false")
	case c == 'n':
		return s.literal("null")
	default:
		return errInvalidJSON
	}
}

func (s *scanner) object(depth int, conv bool) error {
	if depth >= maxDepth {
		return errInvalidJSON
	}
	s.out = append(s.out, '{')
	s.pos++
	s.ws()
	if s.peek() == '}' {
		s.out = append(s.out, '}')
		s.pos++
		return nil
	}
	for {
		s.ws()
		if s.peek() != '"' {
			return errInvalidJSON
		}
		excluded, err := s.key(conv)
		if err != nil {
			return err
		}
		s.ws()
		if s.peek() != ':' {
			return errInvalidJSON
		}
		s.out = append(s.out, ':')
		s.pos++
		if err := s.value(depth+1, conv && !excluded); err != nil {
			return err
		}
		s.ws()
		switch s.peek() {
		case ',':
			s.out = append(s.out, ',')
			s.pos++
		case '}':
			s.out = append(s.out, '}')
			s.pos++
			return nil
		default:
			return errInvalidJSON
		}
	}
}

func (s *scanner) array(depth int, conv bool) error {
	if depth >= maxDepth {
		return errInvalidJSON
	}
	s.out = append(s.out, '[')
	s.pos++
	s.ws()
	if s.peek() == ']' {
		s.out = append(s.out, ']')
		s.pos++
		return nil
	}
	for {
		if err := s.value(depth+1, conv); err != nil {
			return err
		}
		s.ws()
		switch s.peek() {
		case ',':
			s.out = append(s.out, ',')
			s.pos++
		case ']':
			s.out = append(s.out, ']')
			s.pos++
			return nil
		default:
			return errInvalidJSON
		}
	}
}

// key scans an object member name. When conv is false (inside an excluded
// subtree) the name is copied verbatim. Otherwise it is converted, unless it
// matches an exclusion — in which case it is copied verbatim and the returned
// bool tells the caller to leave the member's value untouched as well.
func (s *scanner) key(conv bool) (excluded bool, err error) {
	if !conv {
		return false, s.copyString()
	}

	start := s.pos
	s.pos++ // opening quote
	contentStart := s.pos
	simple := true // pure ASCII, no escapes -> can transform bytes directly
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if c == '"' {
			break
		}
		if c == '\\' {
			simple = false
			s.pos += 2
			continue
		}
		if c < 0x20 {
			return false, errInvalidJSON
		}
		if c >= 0x80 {
			simple = false
		}
		s.pos++
	}
	if s.pos >= len(s.src) {
		return false, errInvalidJSON
	}
	raw := s.src[contentStart:s.pos]
	s.pos++ // closing quote
	token := s.src[start:s.pos]

	if simple {
		if len(s.exclude) > 0 {
			if _, skip := s.exclude[canonicalKey(string(raw))]; skip {
				s.out = append(s.out, token...)
				return true, nil
			}
		}
		s.out = append(s.out, '"')
		s.out = appendConverted(s.out, s.style, raw)
		s.out = append(s.out, '"')
		return false, nil
	}

	// Escapes or non-ASCII: decode, convert with the rune-aware functions, and
	// re-encode as a valid JSON string. Rare, so the extra allocation is fine.
	name, ok := unescapeJSONString(raw)
	if !ok {
		return false, errInvalidJSON
	}
	if len(s.exclude) > 0 {
		if _, skip := s.exclude[canonicalKey(name)]; skip {
			s.out = append(s.out, token...)
			return true, nil
		}
	}
	enc, err := json.Marshal(convertString(s.style, name))
	if err != nil {
		return false, err
	}
	s.out = append(s.out, enc...)
	return false, nil
}

// copyString copies a complete JSON string token (quotes included) verbatim,
// correctly skipping escape sequences so an escaped quote does not end it.
func (s *scanner) copyString() error {
	start := s.pos
	s.pos++ // opening quote
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '\\':
			s.pos += 2
		case c == '"':
			s.pos++
			s.out = append(s.out, s.src[start:s.pos]...)
			return nil
		case c < 0x20:
			return errInvalidJSON
		default:
			s.pos++
		}
	}
	return errInvalidJSON
}

// number validates and copies a JSON number verbatim.
func (s *scanner) number() error {
	start := s.pos
	if s.peek() == '-' {
		s.pos++
	}
	switch {
	case s.peek() == '0':
		s.pos++
	case s.peek() >= '1' && s.peek() <= '9':
		for isDigit(s.peek()) {
			s.pos++
		}
	default:
		return errInvalidJSON
	}
	if s.peek() == '.' {
		s.pos++
		if !isDigit(s.peek()) {
			return errInvalidJSON
		}
		for isDigit(s.peek()) {
			s.pos++
		}
	}
	if e := s.peek(); e == 'e' || e == 'E' {
		s.pos++
		if p := s.peek(); p == '+' || p == '-' {
			s.pos++
		}
		if !isDigit(s.peek()) {
			return errInvalidJSON
		}
		for isDigit(s.peek()) {
			s.pos++
		}
	}
	s.out = append(s.out, s.src[start:s.pos]...)
	return nil
}

func (s *scanner) literal(lit string) error {
	if s.pos+len(lit) <= len(s.src) && string(s.src[s.pos:s.pos+len(lit)]) == lit {
		s.out = append(s.out, lit...)
		s.pos += len(lit)
		return nil
	}
	return errInvalidJSON
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// appendConverted transforms an ASCII key (no escapes) directly into dst,
// allocating nothing beyond dst's own growth.
func appendConverted(dst []byte, style keyStyle, key []byte) []byte {
	if style == styleSnakeToCamel {
		return appendSnakeToCamel(dst, key)
	}
	return appendCamelToSnake(dst, key)
}

// appendCamelToSnake mirrors CamelToSnake for ASCII input, byte for byte.
func appendCamelToSnake(dst, s []byte) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				p := s[i-1]
				if (p >= 'a' && p <= 'z') || (p >= '0' && p <= '9') {
					dst = append(dst, '_')
				} else if p >= 'A' && p <= 'Z' && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
					dst = append(dst, '_')
				}
			}
			dst = append(dst, c+('a'-'A'))
			continue
		}
		dst = append(dst, c)
	}
	return dst
}

// appendSnakeToCamel mirrors SnakeToCamel for ASCII input, byte for byte.
func appendSnakeToCamel(dst, s []byte) []byte {
	upperNext := false
	wrote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			if !wrote {
				dst = append(dst, '_') // preserve leading underscore
				wrote = true
				continue
			}
			upperNext = true
			continue
		}
		if upperNext {
			dst = append(dst, toUpperASCII(c))
			upperNext = false
		} else {
			dst = append(dst, c)
		}
		wrote = true
	}
	if upperNext {
		dst = append(dst, '_') // trailing underscore had no letter to fold in
	}
	return dst
}

func toUpperASCII(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

// convertString applies the rune-aware conversion for the given style, used on
// the rare slow path for keys containing escapes or non-ASCII characters.
func convertString(style keyStyle, s string) string {
	if style == styleSnakeToCamel {
		return SnakeToCamel(s)
	}
	return CamelToSnake(s)
}

// unescapeJSONString decodes the body of a JSON string (the bytes between the
// quotes) into its literal value, using the standard library for correctness.
func unescapeJSONString(raw []byte) (string, bool) {
	buf := make([]byte, 0, len(raw)+2)
	buf = append(buf, '"')
	buf = append(buf, raw...)
	buf = append(buf, '"')
	var out string
	if err := json.Unmarshal(buf, &out); err != nil {
		return "", false
	}
	return out, true
}
