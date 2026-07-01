package chiflex

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ikshantshukla123/jsonflex"
)

func TestChiBidirectional(t *testing.T) {
	var seen string
	r := chi.NewRouter()
	r.Use(Middleware())
	r.Post("/users", func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		seen = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created_id":1,"user_name":"amy","order_items":[{"item_name":"pen"}]}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"userName":"amy","homeAddress":{"zipCode":"10001"}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertJSONEqual(t, []byte(seen), `{"user_name":"amy","home_address":{"zip_code":"10001"}}`)
	assertJSONEqual(t, w.Body.Bytes(), `{"createdId":1,"userName":"amy","orderItems":[{"itemName":"pen"}]}`)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestChiExclude(t *testing.T) {
	var seen string
	r := chi.NewRouter()
	r.Use(Middleware(jsonflex.Exclude("rawMeta"), jsonflex.WithResponseConversion(false)))
	r.Post("/x", func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		seen = string(body)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"userName":"amy","rawMeta":{"keepThis":1}}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	assertJSONEqual(t, []byte(seen), `{"user_name":"amy","rawMeta":{"keepThis":1}}`)
}

func assertJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, wv any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not valid JSON: %v (%s)", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wv); err != nil {
		t.Fatalf("want is not valid JSON: %v", err)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(wv)
	if string(gb) != string(wb) {
		t.Errorf("JSON mismatch:\n got:  %s\n want: %s", got, want)
	}
}
