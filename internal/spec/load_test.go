package spec

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Swagger 2.0 is the big compatibility win: the openapi3 parser cannot read it
// directly, so Load must detect and convert it.
func TestLoadSwagger2(t *testing.T) {
	api, err := Load("../../testdata/swagger2.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if api.Title != "Legacy Store" {
		t.Errorf("title = %q", api.Title)
	}
	// host + basePath + schemes must become a v3 server URL.
	if len(api.Servers) != 1 || api.Servers[0] != "https://api.legacy.test/v2" {
		t.Errorf("servers = %v", api.Servers)
	}
	if len(api.Endpoints) != 4 {
		t.Fatalf("expected 4 endpoints, got %d", len(api.Endpoints))
	}

	get := find(t, api, "GET", "/items")
	var sawStatus, sawLimit bool
	for _, p := range get.Params {
		switch p.Name {
		case "status":
			sawStatus = true
			if !p.Required || len(p.Enum) != 2 {
				t.Errorf("status param = %+v", p)
			}
		case "limit":
			sawLimit = true
			if p.Type != "integer" {
				t.Errorf("limit type = %q", p.Type)
			}
		}
	}
	if !sawStatus || !sawLimit {
		t.Errorf("query params lost in conversion: %+v", get.Params)
	}

	// The body $ref must resolve into a usable skeleton.
	post := find(t, api, "POST", "/items")
	if post.BodySkeleton == "" {
		t.Fatal("expected a body skeleton for POST /items")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(post.BodySkeleton), &body); err != nil {
		t.Fatalf("skeleton not valid json: %v", err)
	}
	if _, ok := body["name"]; !ok {
		t.Errorf("converted body missing name field: %v", body)
	}

	// Path-level parameters must survive the conversion too.
	del := find(t, api, "DELETE", "/items/{itemId}")
	if len(del.Params) != 1 || del.Params[0].Name != "itemId" {
		t.Errorf("path param lost: %+v", del.Params)
	}
}

func TestLoadYAML(t *testing.T) {
	api, err := Load("../../testdata/mini.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if api.Title != "Yaml Service" {
		t.Errorf("title = %q", api.Title)
	}
	if len(api.Endpoints) != 1 || api.Endpoints[0].Path != "/health" {
		t.Errorf("endpoints = %+v", api.Endpoints)
	}
}

func TestLoadRemoteSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"openapi":"3.0.0","info":{"title":"Remote","version":"1"},
			"servers":[{"url":"https://remote.test"}],
			"paths":{"/ping":{"get":{"summary":"ping"}}}}`))
	}))
	defer srv.Close()

	api, err := Load(srv.URL)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if api.Title != "Remote" || len(api.Endpoints) != 1 {
		t.Errorf("remote spec = %+v", api)
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"empty source": "",
		"missing file": "../../testdata/does-not-exist.json",
		"bad scheme":   "ftp://example.com/spec.json",
	}
	for name, src := range cases {
		if _, err := Load(src); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadRejectsNonSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>Swagger UI</body></html>"))
	}))
	defer srv.Close()

	_, err := Load(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "spec") {
		t.Errorf("expected a spec error for an HTML page, got %v", err)
	}
}
