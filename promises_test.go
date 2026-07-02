package jsonflex

// This file verifies, promise by promise, everything the README claims:
// correctness of conversion, recursion, value fidelity, safe passthrough,
// options, and the efficiency/allocation characteristics. Test names are
// prefixed TestPromise_ and grouped by the guarantee they pin down.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// --- Promise: key conversion, including acronyms and idempotency ---

func TestPromise_KeyConversion(t *testing.T) {
	camel := map[string]string{
		"userName": "user_name",
		"userID":   "user_id",
		"APIKey":   "api_key",
		"isHTTPS":  "is_https",
		"id":       "id",
	}
	for in, want := range camel {
		if got := CamelToSnake(in); got != want {
			t.Errorf("CamelToSnake(%q) = %q, want %q", in, got, want)
		}
	}
	snake := map[string]string{
		"user_name": "userName",
		"user_id":   "userId",
		"api_key":   "apiKey",
		"_id":       "_id",
	}
	for in, want := range snake {
		if got := SnakeToCamel(in); got != want {
			t.Errorf("SnakeToCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPromise_ConversionIsIdempotent(t *testing.T) {
	for _, s := range []string{"userName", "userID", "APIKey", "user_name", "already_snake"} {
		if once, twice := CamelToSnake(s), CamelToSnake(CamelToSnake(s)); once != twice {
			t.Errorf("CamelToSnake not idempotent: %q -> %q -> %q", s, once, twice)
		}
	}
	for _, s := range []string{"user_name", "userName", "user_id", "alreadyCamel"} {
		if once, twice := SnakeToCamel(s), SnakeToCamel(SnakeToCamel(s)); once != twice {
			t.Errorf("SnakeToCamel not idempotent: %q -> %q -> %q", s, once, twice)
		}
	}
}

// --- Promise: recursion over nested objects and arrays to any depth ---

func TestPromise_RecursiveNestedAndArrays(t *testing.T) {
	in := `{"userName":"amy","address":{"streetName":"5th","geo":{"latValue":1}},"orderItems":[{"itemName":"pen","tags":["aTag"]}]}`
	got, err := ToSnakeCase([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, got, `{"user_name":"amy","address":{"street_name":"5th","geo":{"lat_value":1}},"order_items":[{"item_name":"pen","tags":["aTag"]}]}`)
}

func TestPromise_DeepNesting(t *testing.T) {
	const depth = 50
	in := strings.Repeat(`{"deepKey":`, depth) + `1` + strings.Repeat(`}`, depth)
	got, err := ToSnakeCase([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "deepKey") {
		t.Error("a nested key was left unconverted")
	}
	if n := strings.Count(string(got), "deep_key"); n != depth {
		t.Errorf("converted %d keys, want %d", n, depth)
	}
}

func TestPromise_LargeArray(t *testing.T) {
	const n = 100
	elems := make([]string, n)
	for i := range elems {
		elems[i] = `{"itemName":"x"}`
	}
	in := "[" + strings.Join(elems, ",") + "]"
	got, err := ToSnakeCase([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "itemName") {
		t.Error("not every array element was converted")
	}
	if c := strings.Count(string(got), "item_name"); c != n {
		t.Errorf("got %d conversions, want %d", c, n)
	}
}

// --- Promise: value fidelity (precision, HTML, all types, key order) ---

func TestPromise_AllValueTypesPreserved(t *testing.T) {
	in := `{"strVal":"hi","intVal":-7,"floatVal":3.14,"expVal":1.5e10,"boolT":true,"boolF":false,"nullV":null,"arrV":[1,"two",false,null]}`
	got, err := ToSnakeCase([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, got, `{"str_val":"hi","int_val":-7,"float_val":3.14,"exp_val":1.5e10,"bool_t":true,"bool_f":false,"null_v":null,"arr_v":[1,"two",false,null]}`)
}

func TestPromise_BigIntegerPrecisionExact(t *testing.T) {
	// 2^63-1 and a value past float64's safe integer range must survive exactly.
	got, err := ToSnakeCase([]byte(`{"maxID":9223372036854775807,"bigID":9007199254740993}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"max_id":9223372036854775807,"big_id":9007199254740993}`; string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestPromise_HTMLNotEscaped(t *testing.T) {
	got, err := ToSnakeCase([]byte(`{"htmlBody":"<p>a & b</p>"}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"html_body":"<p>a & b</p>"}`; string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestPromise_KeyOrderPreserved(t *testing.T) {
	got, err := ToSnakeCase([]byte(`{"zebraField":1,"appleField":2,"middleField":3}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"zebra_field":1,"apple_field":2,"middle_field":3}`; string(got) != want {
		t.Errorf("got %s, want %s (order not preserved)", got, want)
	}
}

func TestPromise_UnicodeKeys(t *testing.T) {
	got, err := ToSnakeCase([]byte(`{"caféKey":1,"naïveField":2}`))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, got, `{"café_key":1,"naïve_field":2}`)
}

func TestPromise_RoundTrip(t *testing.T) {
	orig := `{"user_name":"amy","order_items":[{"item_name":"pen"}]}`
	camel, err := ToCamelCase([]byte(orig))
	if err != nil {
		t.Fatal(err)
	}
	back, err := ToSnakeCase(camel)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, back, orig)
}

// --- Promise: safe by default (content-type gating, passthrough) ---

func TestPromise_MiddlewareContentTypeGating(t *testing.T) {
	cases := []struct {
		ct      string
		convert bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"application/vnd.api+json", true},
		{"text/plain", false},
		{"application/x-www-form-urlencoded", false},
		{"", false},
	}
	for _, tc := range cases {
		var seen string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			seen = string(b)
		})
		srv := Middleware(WithResponseConversion(false))(h)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userName":"amy"}`))
		if tc.ct != "" {
			req.Header.Set("Content-Type", tc.ct)
		}
		srv.ServeHTTP(httptest.NewRecorder(), req)

		want := `{"userName":"amy"}`
		if tc.convert {
			want = `{"user_name":"amy"}`
		}
		if seen != want {
			t.Errorf("content-type %q: got %s, want %s", tc.ct, seen, want)
		}
	}
}

func TestPromise_InvalidAndEmptyPassThrough(t *testing.T) {
	for _, payload := range []string{`{"userName": not-valid}`, `{unbalanced`, ``, `   `} {
		var seen string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			seen = string(b)
			w.WriteHeader(http.StatusOK)
		})
		srv := Middleware(WithResponseConversion(false))(h)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if seen != payload {
			t.Errorf("payload %q was altered to %q", payload, seen)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("payload %q caused status %d, want 200 (must not error)", payload, rec.Code)
		}
	}
}

// --- Promise: full bidirectional flow through a real router ---

func TestPromise_BidirectionalThroughMux(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, ok := in["user_name"]; !ok {
			t.Errorf("handler did not receive snake_case keys: %v", in)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created_id":1,"user_name":"amy"}`))
	})
	srv := Middleware()(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"userName":"amy","homeAddress":{"zipCode":"10001"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"createdId":1,"userName":"amy"}`)
}

// --- Promise: options (toggles, custom funcs, exclude both directions) ---

func TestPromise_DirectionToggles(t *testing.T) {
	reqOnly := New(WithResponseConversion(false))
	if reqOnly.ResponseConversionEnabled() {
		t.Error("response conversion should be disabled")
	}
	if got := string(reqOnly.ConvertResponseBody([]byte(`{"user_name":"a"}`))); got != `{"user_name":"a"}` {
		t.Errorf("disabled response conversion changed body: %s", got)
	}

	respOnly := New(WithRequestConversion(false))
	if got := string(respOnly.ConvertRequestBody([]byte(`{"userName":"a"}`))); got != `{"userName":"a"}` {
		t.Errorf("disabled request conversion changed body: %s", got)
	}
}

func TestPromise_CustomKeyFunc(t *testing.T) {
	c := New(WithRequestKeyFunc(strings.ToUpper), WithResponseConversion(false))
	got := c.ConvertRequestBody([]byte(`{"userName":"amy","nested":{"aKey":1}}`))
	assertJSONEqual(t, got, `{"USERNAME":"amy","NESTED":{"AKEY":1}}`)
}

func TestPromise_ExcludeProtectsBothDirectionsViaMiddleware(t *testing.T) {
	// Request side: camelCase key protected.
	var seen string
	reqH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
	})
	reqSrv := Middleware(WithResponseConversion(false), Exclude("rawMeta"))(reqH)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userName":"amy","rawMeta":{"keepThis":1}}`))
	req.Header.Set("Content-Type", "application/json")
	reqSrv.ServeHTTP(httptest.NewRecorder(), req)
	assertJSONEqual(t, []byte(seen), `{"user_name":"amy","rawMeta":{"keepThis":1}}`)

	// Response side: the SAME exclusion protects the snake_case key.
	respH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user_name":"amy","raw_meta":{"keep_this":1}}`))
	})
	respSrv := Middleware(WithRequestConversion(false), Exclude("rawMeta"))(respH)
	rec := httptest.NewRecorder()
	respSrv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assertJSONEqual(t, rec.Body.Bytes(), `{"userName":"amy","raw_meta":{"keep_this":1}}`)
}

// --- Promise: efficient (byte engine allocations) ---

func TestPromise_ByteEngineMinimalAllocations(t *testing.T) {
	avg := testing.AllocsPerRun(200, func() {
		_, _ = ToSnakeCase(benchPayload)
	})
	if avg > 2 {
		t.Errorf("ToSnakeCase averaged %.1f allocs/op; want <= 2 (byte-engine regression)", avg)
	}
}

func TestPromise_ByteEngineFarFewerAllocsThanTreeWalk(t *testing.T) {
	fast := testing.AllocsPerRun(100, func() { _, _ = transformBytes(benchPayload, styleCamelToSnake, nil) })
	tree := testing.AllocsPerRun(100, func() { _, _ = transform(benchPayload, CamelToSnake, nil) })
	if fast >= tree {
		t.Errorf("byte engine (%.0f allocs) should be far below tree-walk (%.0f allocs)", fast, tree)
	}
	t.Logf("allocations/op — byte engine: %.0f, tree-walk: %.0f", fast, tree)
}

// --- Promise: Converter is safe for concurrent use ---

func TestPromise_ConverterConcurrentSafe(t *testing.T) {
	c := New()
	const in = `{"userName":"amy","orderItems":[{"itemName":"pen"}]}`
	const want = `{"user_name":"amy","order_items":[{"item_name":"pen"}]}`

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if got := string(c.ConvertRequestBody([]byte(in))); got != want {
					t.Errorf("concurrent conversion = %s", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}
