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
