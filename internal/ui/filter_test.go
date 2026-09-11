package ui

import (
	"strings"
	"testing"
)

const filterBody = `{
  "meta": {"total": 42},
  "pets": [
    {"id": 1, "name": "Rex"},
    {"id": 2, "name": "Fido"}
  ]
}`

func TestApplyDotPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{".meta.total", "42"},
		{".pets[0].name", `"Rex"`},
		{".pets[1].id", "2"},
		{".", `"total": 42`}, // whole doc contains this
		{"", `"pets"`},
	}
	for _, c := range cases {
		got, err := applyDotPath(filterBody, c.path)
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.path, err)
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%q: got %q, want it to contain %q", c.path, got, c.want)
		}
	}
}

func TestExtractValue(t *testing.T) {
	body := `{"token":"abc","id":7,"ok":true,"pi":3.5,"nil":null,"obj":{"a":1},"arr":[1,2]}`
	cases := map[string]string{
		".token": "abc",     // string, NO quotes
		".id":    "7",       // integral number, no ".0"
		".ok":    "true",    // bool
		".pi":    "3.5",     // float
		".nil":   "",        // null -> empty
		".obj":   `{"a":1}`, // object -> compact JSON
		".arr":   `[1,2]`,   // array -> compact JSON
	}
	for path, want := range cases {
		got, err := extractValue(body, path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if got != want {
			t.Errorf("extractValue(%s) = %q, want %q", path, got, want)
		}
	}

	if _, err := extractValue(body, ".nope"); err == nil {
		t.Error("expected error for missing key")
	}
	if _, err := extractValue("not json", ".a"); err == nil {
		t.Error("expected error for non-JSON")
	}
}

func TestApplyDotPathErrors(t *testing.T) {
	if _, err := applyDotPath(filterBody, ".nope"); err == nil {
		t.Error("expected error for missing key")
	}
	if _, err := applyDotPath(filterBody, ".pets[9]"); err == nil {
		t.Error("expected error for out-of-range index")
	}
	if _, err := applyDotPath("not json", ".a"); err == nil {
		t.Error("expected error for non-JSON body")
	}
	if _, err := applyDotPath(filterBody, "meta"); err == nil {
		t.Error("expected error for path not starting with '.'")
	}
}
