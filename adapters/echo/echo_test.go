package echoflex

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ikshantshukla123/jsonflex"
	"github.com/labstack/echo/v4"
)

func TestEchoBidirectional(t *testing.T) {
	var seen string
	e := echo.New()
	e.Use(Middleware())
	e.POST("/users", func(c echo.Context) error {
		body, _ := io.ReadAll(c.Request().Body)
		seen = string(body)
		return c.JSON(http.StatusCreated, map[string]any{
			"created_id":  1,
			"user_name":   "amy",
			"order_items": []map[string]any{{"item_name": "pen"}},
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"userName":"amy","homeAddress":{"zipCode":"10001"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assertJSONEqual(t, []byte(seen), `{"user_name":"amy","home_address":{"zip_code":"10001"}}`)
	assertJSONEqual(t, rec.Body.Bytes(), `{"createdId":1,"userName":"amy","orderItems":[{"itemName":"pen"}]}`)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestEchoExclude(t *testing.T) {
	var seen string
	e := echo.New()
	e.Use(Middleware(jsonflex.Exclude("rawMeta"), jsonflex.WithResponseConversion(false)))
	e.POST("/x", func(c echo.Context) error {
		body, _ := io.ReadAll(c.Request().Body)
		seen = string(body)
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"userName":"amy","rawMeta":{"keepThis":1}}`))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(httptest.NewRecorder(), req)

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
