package ginflex

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ikshantshukla123/jsonflex"
)

func init() { gin.SetMode(gin.TestMode) }

func TestGinBidirectional(t *testing.T) {
	var seen string
	r := gin.New()
	r.Use(Middleware())
	r.POST("/users", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		seen = string(body)
		c.JSON(http.StatusCreated, gin.H{"created_id": 1, "user_name": "amy", "order_items": []gin.H{{"item_name": "pen"}}})
	})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"userName":"amy","homeAddress":{"zipCode":"10001"}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Handler saw snake_case.
	assertJSONEqual(t, []byte(seen), `{"user_name":"amy","home_address":{"zip_code":"10001"}}`)
	// Client got camelCase.
	assertJSONEqual(t, w.Body.Bytes(), `{"createdId":1,"userName":"amy","orderItems":[{"itemName":"pen"}]}`)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestGinExclude(t *testing.T) {
	var seen string
	r := gin.New()
	r.Use(Middleware(jsonflex.Exclude("rawMeta"), jsonflex.WithResponseConversion(false)))
	r.POST("/x", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		seen = string(body)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"userName":"amy","rawMeta":{"keepThis":1}}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	assertJSONEqual(t, []byte(seen), `{"user_name":"amy","rawMeta":{"keepThis":1}}`)
}

func TestGinIgnoresNonJSON(t *testing.T) {
	r := gin.New()
	r.Use(Middleware())
	r.GET("/plain", func(c *gin.Context) {
		c.String(http.StatusOK, "orderItems stay put")
	})

	req := httptest.NewRequest(http.MethodGet, "/plain", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "orderItems stay put" {
		t.Errorf("non-JSON response altered: %q", w.Body.String())
	}
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
