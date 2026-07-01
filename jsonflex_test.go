package jsonflex

import (
	"encoding/json"
	"testing"
)

func TestCamelToSnake(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"id":            "id",
		"userName":      "user_name",
		"UserName":      "user_name",
		"userID":        "user_id",
		"APIKey":        "api_key",
		"isHTTPS":       "is_https",
		"already_snake": "already_snake",
		"user2Name":     "user2_name",
		"HTTP":          "http",
		"a":             "a",
	}
	for in, want := range cases {
		if got := CamelToSnake(in); got != want {
			t.Errorf("CamelToSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnakeToCamel(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"id":           "id",
		"user_name":    "userName",
		"user_id":      "userId",
		"api_key":      "apiKey",
		"_id":          "_id",
		"alreadyCamel": "alreadyCamel",
		"a":            "a",
		"trailing_":    "trailing_",
	}
	for in, want := range cases {
		if got := SnakeToCamel(in); got != want {
			t.Errorf("SnakeToCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToSnakeCaseNested(t *testing.T) {
	in := []byte(`{"userName":"amy","address":{"streetName":"5th Ave","zipCode":"10001"},"orderItems":[{"itemName":"pen","unitPrice":2}]}`)
	got, err := ToSnakeCase(in)
	if err != nil {
		t.Fatalf("ToSnakeCase error: %v", err)
	}
	assertJSONEqual(t, got, `{"user_name":"amy","address":{"street_name":"5th Ave","zip_code":"10001"},"order_items":[{"item_name":"pen","unit_price":2}]}`)
}

func TestToCamelCaseNested(t *testing.T) {
	in := []byte(`{"user_name":"amy","order_items":[{"item_name":"pen"}]}`)
	got, err := ToCamelCase(in)
	if err != nil {
		t.Fatalf("ToCamelCase error: %v", err)
	}
	assertJSONEqual(t, got, `{"userName":"amy","orderItems":[{"itemName":"pen"}]}`)
}

func TestTransformTopLevelArray(t *testing.T) {
	in := []byte(`[{"firstName":"a"},{"firstName":"b"}]`)
	got, err := ToSnakeCase(in)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	assertJSONEqual(t, got, `[{"first_name":"a"},{"first_name":"b"}]`)
}

func TestTransformPreservesNumberPrecision(t *testing.T) {
	// A value that would lose precision if decoded as float64.
	in := []byte(`{"bigID":123456789012345678}`)
	got, err := ToSnakeCase(in)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	assertJSONEqual(t, got, `{"big_id":123456789012345678}`)
}

func TestTransformDoesNotEscapeHTML(t *testing.T) {
	in := []byte(`{"htmlBody":"<p>a & b</p>"}`)
	got, err := ToSnakeCase(in)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if string(got) != `{"html_body":"<p>a & b</p>"}` {
		t.Errorf("got %s, want unescaped HTML", got)
	}
}

func TestTransformInvalidJSON(t *testing.T) {
	if _, err := ToSnakeCase([]byte(`{not json`)); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestTransformScalarAndNull(t *testing.T) {
	for _, in := range []string{`"hello"`, `42`, `true`, `null`} {
		got, err := ToSnakeCase([]byte(in))
		if err != nil {
			t.Fatalf("ToSnakeCase(%q) error: %v", in, err)
		}
		assertJSONEqual(t, got, in)
	}
}

func TestExcludeLeavesSubtreeUntouched(t *testing.T) {
	in := []byte(`{"userName":"amy","rawMeta":{"keepThis":1,"andThis":2}}`)
	got, err := transform(in, CamelToSnake, map[string]struct{}{"raw_meta": {}})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	assertJSONEqual(t, got, `{"user_name":"amy","rawMeta":{"keepThis":1,"andThis":2}}`)
}

// assertJSONEqual compares two JSON documents for semantic equality (ignoring
// key order), failing the test with a readable message on mismatch.
func assertJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not valid JSON: %v (%s)", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("want is not valid JSON: %v", err)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	if string(gb) != string(wb) {
		t.Errorf("JSON mismatch:\n got:  %s\n want: %s", got, want)
	}
}
