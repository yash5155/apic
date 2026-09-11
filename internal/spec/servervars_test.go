package spec

import "testing"

func TestServerVariableExpansion(t *testing.T) {
	api, err := Load("../../testdata/servervars.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(api.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %v", api.Servers)
	}
	if api.Servers[0] != "https://api.example.com:443/v2" {
		t.Errorf("server[0] = %q, want fully expanded", api.Servers[0])
	}
	// A variable with no default/enum leaves its token in place.
	if api.Servers[1] != "https://{tenant}.example.com" {
		t.Errorf("server[1] = %q, want unfilled token retained", api.Servers[1])
	}
}
