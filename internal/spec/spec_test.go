package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func load(t *testing.T) *API {
	t.Helper()
	api, err := Load("../../testdata/petstore.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return api
}

func find(t *testing.T, api *API, method, path string) Endpoint {
	t.Helper()
	for _, ep := range api.Endpoints {
		if ep.Method == method && ep.Path == path {
			return ep
		}
	}
	t.Fatalf("endpoint %s %s not found", method, path)
	return Endpoint{}
}

func TestLoadBasics(t *testing.T) {
	api := load(t)
	if api.Title != "Petstore" {
		t.Errorf("title = %q", api.Title)
	}
	if len(api.Servers) != 1 || api.Servers[0] != "https://api.petstore.test/v1" {
		t.Errorf("servers = %v", api.Servers)
	}
	if len(api.Endpoints) != 4 {
		t.Errorf("expected 4 endpoints, got %d", len(api.Endpoints))
	}
}

// Path-level parameters must be merged into every operation under them.
func TestPathLevelParamsAreMerged(t *testing.T) {
	ep := find(t, api(t), "GET", "/pets")

	var names []string
	for _, p := range ep.Params {
		names = append(names, p.Name+":"+p.In)
	}
	joined := strings.Join(names, ",")

	if !strings.Contains(joined, "X-Request-Id:header") {
		t.Errorf("path-level header param missing, got %s", joined)
	}
	if !strings.Contains(joined, "limit:query") || !strings.Contains(joined, "status:query") {
		t.Errorf("operation params missing, got %s", joined)
	}
}

func TestEnumAndDefaultAreExtracted(t *testing.T) {
	ep := find(t, api(t), "GET", "/pets")
	for _, p := range ep.Params {
		switch p.Name {
		case "status":
			if len(p.Enum) != 3 || p.Enum[0] != "available" {
				t.Errorf("status enum = %v", p.Enum)
			}
			if !p.Required {
				t.Error("status should be required")
			}
		case "limit":
			if p.Type != "integer" {
				t.Errorf("limit type = %q", p.Type)
			}
			if p.Default != "20" {
				t.Errorf("limit default = %q", p.Default)
			}
		}
	}
}

// The POST body uses $ref, including a nested $ref. Resolving both is the
// main reason we lean on kin-openapi instead of parsing the JSON ourselves.
func TestBodySkeletonResolvesRefs(t *testing.T) {
	ep := find(t, api(t), "POST", "/pets")
	if ep.BodySkeleton == "" {
		t.Fatal("expected a generated body skeleton")
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(ep.BodySkeleton), &got); err != nil {
		t.Fatalf("skeleton is not valid json: %v\n%s", err, ep.BodySkeleton)
	}

	if _, ok := got["name"]; !ok {
		t.Error("missing name field")
	}
	if v, ok := got["age"].(float64); !ok || v != 0 {
		t.Errorf("age should default to 0, got %v", got["age"])
	}
	if v, ok := got["vaccinated"].(bool); !ok || v != false {
		t.Errorf("vaccinated should default to false, got %v", got["vaccinated"])
	}
	if _, ok := got["tags"].([]any); !ok {
		t.Errorf("tags should be an array, got %T", got["tags"])
	}

	// nested $ref
	owner, ok := got["owner"].(map[string]any)
	if !ok {
		t.Fatalf("owner should be an object, got %T", got["owner"])
	}
	if _, ok := owner["email"]; !ok {
		t.Error("nested Owner ref did not resolve")
	}
}

func TestGetEndpointHasNoBody(t *testing.T) {
	if ep := find(t, api(t), "GET", "/pets"); ep.BodySkeleton != "" {
		t.Errorf("GET should have no body, got %q", ep.BodySkeleton)
	}
}

// tiny helper so each test doesn't re-declare it
func api(t *testing.T) *API { return load(t) }
