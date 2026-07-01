package jsonflex

import "testing"

func TestConverterRequestResponse(t *testing.T) {
	c := New()
	if !c.RequestConversionEnabled() || !c.ResponseConversionEnabled() {
		t.Fatal("expected both directions enabled by default")
	}

	req := c.ConvertRequestBody([]byte(`{"userName":"amy","orderItems":[{"itemName":"pen"}]}`))
	assertJSONEqual(t, req, `{"user_name":"amy","order_items":[{"item_name":"pen"}]}`)

	resp := c.ConvertResponseBody([]byte(`{"user_name":"amy"}`))
	assertJSONEqual(t, resp, `{"userName":"amy"}`)
}

func TestConverterDisabledDirections(t *testing.T) {
	c := New(WithRequestConversion(false), WithResponseConversion(false))
	if c.RequestConversionEnabled() || c.ResponseConversionEnabled() {
		t.Fatal("expected both directions disabled")
	}

	const in = `{"userName":"amy"}`
	if got := string(c.ConvertRequestBody([]byte(in))); got != in {
		t.Errorf("disabled request conversion changed body: %s", got)
	}
	if got := string(c.ConvertResponseBody([]byte(in))); got != in {
		t.Errorf("disabled response conversion changed body: %s", got)
	}
}

func TestConverterPassthrough(t *testing.T) {
	c := New()
	for _, in := range []string{"", "   ", `{not json`, `not json at all`} {
		if got := string(c.ConvertRequestBody([]byte(in))); got != in {
			t.Errorf("ConvertRequestBody(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestConverterExclude(t *testing.T) {
	c := New(Exclude("rawMeta"))
	got := c.ConvertRequestBody([]byte(`{"userName":"amy","rawMeta":{"keepThis":1}}`))
	assertJSONEqual(t, got, `{"user_name":"amy","rawMeta":{"keepThis":1}}`)
}

// TestConverterExcludeIsDirectionAgnostic pins the fix for the exclusion
// footgun: a single Exclude("rawMeta") must protect the field on BOTH the
// request side (where the key is camelCase "rawMeta") and the response side
// (where the same field is snake_case "raw_meta").
func TestConverterExcludeIsDirectionAgnostic(t *testing.T) {
	c := New(Exclude("rawMeta"))

	req := c.ConvertRequestBody([]byte(`{"userName":"amy","rawMeta":{"keepThis":1}}`))
	assertJSONEqual(t, req, `{"user_name":"amy","rawMeta":{"keepThis":1}}`)

	resp := c.ConvertResponseBody([]byte(`{"user_name":"amy","raw_meta":{"keep_this":1}}`))
	assertJSONEqual(t, resp, `{"userName":"amy","raw_meta":{"keep_this":1}}`)
}

// TestConverterExcludeAcceptsEitherForm shows that passing the snake_case name
// is equivalent to passing the camelCase name.
func TestConverterExcludeAcceptsEitherForm(t *testing.T) {
	c := New(Exclude("raw_meta"))

	req := c.ConvertRequestBody([]byte(`{"userName":"amy","rawMeta":{"keepThis":1}}`))
	assertJSONEqual(t, req, `{"user_name":"amy","rawMeta":{"keepThis":1}}`)

	resp := c.ConvertResponseBody([]byte(`{"user_name":"amy","raw_meta":{"keep_this":1}}`))
	assertJSONEqual(t, resp, `{"userName":"amy","raw_meta":{"keep_this":1}}`)
}
