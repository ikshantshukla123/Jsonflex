package jsonflex

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// decodeAny decodes JSON into a generic value using json.Number, so numbers are
// compared by their exact digits rather than lossy float64.
func decodeAny(t *testing.T, data []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return v
}

// corpus is a set of valid JSON documents exercising nesting, arrays, every
// scalar type, escapes, unicode keys, big integers, HTML, and whitespace.
var corpus = []string{
	`{}`,
	`[]`,
	`{"userName":"amy"}`,
	`{"a":{"bNested":[1,2,{"cDeep":true}]}}`,
	`[{"firstName":"a"},{"firstName":"b"}]`,
	`{"htmlBody":"<p>a & b</p>"}`,
	`{"bigID":123456789012345678}`,
	`{"floatVal":1.5e-10,"negVal":-42,"zero":0}`,
	`{"nullVal":null,"boolVal":false,"t":true}`,
	`{"escaped":"line1\nline2\t\"quoted\" \\ \/ é"}`,
	`{"keyWithAEscape":"v"}`,
	`{"caféKey":"value","naïveField":1}`,
	`{ "userName" : "amy" , "nested" : { "aB" : 1 } }`,
	`{"_id":"x","userID":1,"APIKey":"k","isHTTPS":true}`,
	`"top-level string"`,
	`12345`,
	`{"arr":[[[{"deepKey":[null,{"innerMost":1}]}]]]}`,
}

// TestFastPathMatchesTreeWalk is the key correctness guarantee: for every input
// and both built-in directions, the byte-level engine must produce output that
// is semantically identical to the tree-walk engine.
func TestFastPathMatchesTreeWalk(t *testing.T) {
	directions := []struct {
		name  string
		style keyStyle
		fn    KeyFunc
	}{
		{"camelToSnake", styleCamelToSnake, CamelToSnake},
		{"snakeToCamel", styleSnakeToCamel, SnakeToCamel},
	}
	for _, in := range corpus {
		for _, d := range directions {
			fast, ferr := transformBytes([]byte(in), d.style, nil)
			tree, terr := transform([]byte(in), d.fn, nil)
			if ferr != nil || terr != nil {
				t.Fatalf("%s %q: unexpected error fast=%v tree=%v", d.name, in, ferr, terr)
			}
			if !reflect.DeepEqual(decodeAny(t, fast), decodeAny(t, tree)) {
				t.Errorf("%s divergence for %q:\n fast: %s\n tree: %s", d.name, in, fast, tree)
			}
		}
	}
}

func TestFastPathExcludeMatchesTreeWalk(t *testing.T) {
	ex := map[string]struct{}{canonicalKey("rawMeta"): {}}
	in := []byte(`{"userName":"amy","rawMeta":{"keepThis":[1,{"nestedKey":2}]}}`)

	fast, _ := transformBytes(in, styleCamelToSnake, ex)
	tree, _ := transform(in, CamelToSnake, ex)
	if !reflect.DeepEqual(decodeAny(t, fast), decodeAny(t, tree)) {
		t.Errorf("exclude divergence:\n fast: %s\n tree: %s", fast, tree)
	}
	// The excluded subtree (including nestedKey) must be untouched.
	assertJSONEqual(t, fast, `{"user_name":"amy","rawMeta":{"keepThis":[1,{"nestedKey":2}]}}`)
}

func TestFastPathPreservesKeyOrder(t *testing.T) {
	// The tree-walk engine alphabetizes map keys; the byte engine preserves the
	// document's original order. Pin the order-preserving behavior.
	out, err := transformBytes([]byte(`{"zebraField":1,"appleField":2}`), styleCamelToSnake, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), `{"zebra_field":1,"apple_field":2}`; got != want {
		t.Errorf("got %s, want %s (key order not preserved)", got, want)
	}
}

func TestFastPathExactOutput(t *testing.T) {
	cases := map[string]string{
		// Big integer preserved digit-for-digit (no float64 rounding).
		`{"bigID":123456789012345678}`: `{"big_id":123456789012345678}`,
		// HTML not escaped.
		`{"htmlBody":"<p>a & b</p>"}`: `{"html_body":"<p>a & b</p>"}`,
		// Whitespace preserved verbatim.
		`{ "aB" : 1 }`: `{ "a_b" : 1 }`,
	}
	for in, want := range cases {
		got, err := transformBytes([]byte(in), styleCamelToSnake, nil)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if string(got) != want {
			t.Errorf("transformBytes(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestFastPathRejectsInvalidJSON(t *testing.T) {
	invalid := []string{
		``,
		`{`,
		`{"a":}`,
		`{"a" 1}`,
		`[1,]`,
		`{"a":1,}`,
		`"unterminated`,
		`01`,
		`{"a":1}extra`,
		`truex`,
		`[1 2]`,
		`{"a":1 "b":2}`,
	}
	for _, in := range invalid {
		if _, err := transformBytes([]byte(in), styleCamelToSnake, nil); err == nil {
			t.Errorf("expected error for invalid JSON %q, got nil", in)
		}
	}
}

func TestFastPathDepthLimit(t *testing.T) {
	// Within the limit: fine.
	ok := strings.Repeat("[", 1000) + strings.Repeat("]", 1000)
	if _, err := transformBytes([]byte(ok), styleCamelToSnake, nil); err != nil {
		t.Errorf("depth 1000 should succeed, got %v", err)
	}
	// Beyond the limit: rejected rather than overflowing the stack.
	deep := strings.Repeat("[", maxDepth+1) + strings.Repeat("]", maxDepth+1)
	if _, err := transformBytes([]byte(deep), styleCamelToSnake, nil); err == nil {
		t.Error("expected error past max depth, got nil")
	}
}

// --- Benchmarks: byte engine vs tree-walk engine on the same payload ---

func BenchmarkFastPathToSnake(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchPayload)))
	for i := 0; i < b.N; i++ {
		if _, err := transformBytes(benchPayload, styleCamelToSnake, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTreeWalkToSnake(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchPayload)))
	for i := 0; i < b.N; i++ {
		if _, err := transform(benchPayload, CamelToSnake, nil); err != nil {
			b.Fatal(err)
		}
	}
}
