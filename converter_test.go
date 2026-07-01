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
