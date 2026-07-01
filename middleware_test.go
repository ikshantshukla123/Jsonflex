package jsonflex

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareConvertsRequestBody(t *testing.T) {
	var seen string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = string(body)
		w.WriteHeader(http.StatusNoContent)
	})

	srv := Middleware(WithResponseConversion(false))(handler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userName":"amy","address":{"zipCode":"10001"}}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	want := `{"user_name":"amy","address":{"zip_code":"10001"}}`
	assertJSONEqual(t, []byte(seen), want)
}

func TestMiddlewareConvertsResponseBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user_name":"amy","order_items":[{"item_name":"pen"}]}`))
	})

	srv := Middleware(WithRequestConversion(false))(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assertJSONEqual(t, rec.Body.Bytes(), `{"userName":"amy","orderItems":[{"itemName":"pen"}]}`)
	if got := rec.Header().Get("Content-Length"); got == "" {
		t.Error("expected Content-Length to be set on converted response")
	}
}

func TestMiddlewareBidirectional(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Echo the request body straight back; it should already be snake_case
		// coming in, and get converted to camelCase going out.
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})

	srv := Middleware()(handler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"firstName":"amy"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assertJSONEqual(t, rec.Body.Bytes(), `{"firstName":"amy"}`)
}

func TestMiddlewareIgnoresNonJSON(t *testing.T) {
	const payload = `userName=amy`
	var seen string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = string(body)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("orderItems ok"))
	})

	srv := Middleware()(handler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if seen != payload {
		t.Errorf("non-JSON request body was altered: got %q", seen)
	}
	if rec.Body.String() != "orderItems ok" {
		t.Errorf("non-JSON response body was altered: got %q", rec.Body.String())
	}
}

func TestMiddlewarePassesThroughInvalidJSON(t *testing.T) {
	const payload = `{"userName": not-valid}`
	var seen string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = string(body)
	})

	srv := Middleware(WithResponseConversion(false))(handler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	if seen != payload {
		t.Errorf("invalid JSON should pass through unchanged: got %q", seen)
	}
}

func TestMiddlewarePreservesStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"resource_id":7}`))
	})

	srv := Middleware()(handler)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"resourceId":7}`)
}

func TestMiddlewareExclude(t *testing.T) {
	var seen string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = string(body)
	})

	srv := Middleware(WithResponseConversion(false), Exclude("rawMeta"))(handler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userName":"amy","rawMeta":{"keepThis":1}}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	assertJSONEqual(t, []byte(seen), `{"user_name":"amy","rawMeta":{"keepThis":1}}`)
}

func TestMiddlewareEmptyBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) != 0 {
			t.Errorf("expected empty body, got %q", body)
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := Middleware()(handler)
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestIsJSONContentType(t *testing.T) {
	cases := map[string]bool{
		"application/json":                  true,
		"application/json; charset=utf-8":   true,
		"APPLICATION/JSON":                  true,
		"application/vnd.api+json":          true,
		"text/plain":                        false,
		"application/x-www-form-urlencoded": false,
		"":                                  false,
	}
	for ct, want := range cases {
		if got := IsJSONContentType(ct); got != want {
			t.Errorf("IsJSONContentType(%q) = %v, want %v", ct, got, want)
		}
	}
}
