package spec

import (
	"strings"
	"testing"
)

func TestSecuritySchemesCaptured(t *testing.T) {
	api, err := Load("../../testdata/secured.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(api.Security) != 2 {
		t.Fatalf("expected 2 security schemes, got %d: %+v", len(api.Security), api.Security)
	}

	ak := api.Security["ApiKeyAuth"]
	if ak.Type != "apiKey" || ak.In != "header" || ak.Name != "X-API-Key" {
		t.Errorf("apiKey scheme = %+v", ak)
	}

	br := api.Security["BearerAuth"]
	if br.Type != "http" || br.Scheme != "bearer" || br.In != "header" || br.Name != "Authorization" {
		t.Errorf("bearer scheme = %+v", br)
	}
}

func TestEndpointAuthResolved(t *testing.T) {
	api, err := Load("../../testdata/secured.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// GET overrides global security with ApiKeyAuth.
	get := find(t, api, "GET", "/widgets")
	if len(get.Auth) != 1 || get.Auth[0] != "ApiKeyAuth" {
		t.Errorf("GET auth = %v, want [ApiKeyAuth]", get.Auth)
	}

	// POST inherits the global BearerAuth requirement.
	post := find(t, api, "POST", "/widgets")
	if len(post.Auth) != 1 || post.Auth[0] != "BearerAuth" {
		t.Errorf("POST auth = %v, want [BearerAuth]", post.Auth)
	}
}

func TestBodyValidation(t *testing.T) {
	api, err := Load("../../testdata/secured.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	post := find(t, api, "POST", "/widgets")
	if post.ValidateBody == nil {
		t.Fatal("POST /widgets should have a body validator")
	}

	// Missing the required "name" field.
	if msg := post.ValidateBody(`{"size": 3}`); msg == "" || !strings.Contains(msg, "name") {
		t.Errorf("expected a validation error naming 'name', got %q", msg)
	}
	// Wrong type for size.
	if msg := post.ValidateBody(`{"name":"a","size":"big"}`); msg == "" {
		t.Error("expected a type error for size")
	}
	// Valid body.
	if msg := post.ValidateBody(`{"name":"a","size":3}`); msg != "" {
		t.Errorf("valid body rejected: %q", msg)
	}
	// Malformed JSON.
	if msg := post.ValidateBody(`{not json`); msg == "" {
		t.Error("expected a JSON parse error")
	}

	// GET has no JSON body → no validator.
	get := find(t, api, "GET", "/widgets")
	if get.ValidateBody != nil {
		t.Error("GET should have no body validator")
	}
}
