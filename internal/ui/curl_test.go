package ui

import (
	"strings"
	"testing"

	"github.com/yash5155/apic/internal/httpx"
)

func TestBuildCurl(t *testing.T) {
	r := httpx.Request{
		Method:     "POST",
		BaseURL:    "https://api.test/v1",
		Path:       "/pets/{id}",
		PathParams: map[string]string{"id": "7"},
		Query:      map[string]string{"dry": "true"},
		Headers:    map[string]string{"X-Token": "abc"},
		Body:       `{"name":"O'Brien"}`,
	}

	got, err := buildCurl(r)
	if err != nil {
		t.Fatalf("buildCurl: %v", err)
	}

	for _, want := range []string{
		"curl -X POST",
		"https://api.test/v1/pets/7?dry=true",
		"-H 'X-Token: abc'",
		"-H 'Content-Type: application/json'",
		`-d '{"name":"O'\''Brien"}'`, // single quote escaped
	} {
		if !strings.Contains(got, want) {
			t.Errorf("curl missing %q\nfull:\n%s", want, got)
		}
	}
}
