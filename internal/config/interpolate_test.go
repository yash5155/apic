package config

import (
	"reflect"
	"testing"
)

func TestInterpolate(t *testing.T) {
	vars := map[string]string{"host": "api.x", "id": "7"}

	cases := []struct {
		in        string
		wantOut   string
		wantUnres []string
	}{
		{"{{host}}/v1", "api.x/v1", nil},
		{"{{ host }}/{{id}}", "api.x/7", nil},                      // spaced + multiple
		{"/pets/{petId}", "/pets/{petId}", nil},                    // single brace untouched, not reported
		{"{{missing}}", "{{missing}}", []string{"missing"}},        // unknown left verbatim + reported
		{"{{a}}{{a}}{{b}}", "{{a}}{{a}}{{b}}", []string{"a", "b"}}, // de-duped, sorted
		{"plain text", "plain text", nil},
	}
	for _, c := range cases {
		out, unres := Interpolate(c.in, vars)
		if out != c.wantOut {
			t.Errorf("Interpolate(%q) out = %q, want %q", c.in, out, c.wantOut)
		}
		if !reflect.DeepEqual(unres, c.wantUnres) {
			t.Errorf("Interpolate(%q) unresolved = %v, want %v", c.in, unres, c.wantUnres)
		}
	}
}

func TestInterpolateEnvPrefix(t *testing.T) {
	t.Setenv("APIC_TEST_TOKEN", "s3cret")
	out, unres := Interpolate("Bearer {{env.APIC_TEST_TOKEN}}", nil)
	if out != "Bearer s3cret" || unres != nil {
		t.Errorf("out=%q unresolved=%v", out, unres)
	}
	// Missing env var is reported.
	if _, u := Interpolate("{{env.NOPE_XYZ}}", nil); len(u) != 1 || u[0] != "env.NOPE_XYZ" {
		t.Errorf("missing env should be reported, got %v", u)
	}
}

func TestInterpolateMap(t *testing.T) {
	m := map[string]string{"a": "{{x}}", "b": "lit"}
	out, unres := InterpolateMap(m, map[string]string{"x": "1"})
	if out["a"] != "1" || out["b"] != "lit" {
		t.Errorf("out = %v", out)
	}
	if unres != nil {
		t.Errorf("unresolved = %v", unres)
	}
}

func TestMergeVarsPrecedence(t *testing.T) {
	out := MergeVars(map[string]string{"a": "1", "b": "2"}, map[string]string{"b": "9", "c": "3"})
	if out["a"] != "1" || out["b"] != "9" || out["c"] != "3" {
		t.Errorf("merge = %v (--var should win on b)", out)
	}
	if _, ok := out["timestamp"]; !ok {
		t.Error("built-in timestamp should be present")
	}
	// File var can override a built-in.
	out2 := MergeVars(map[string]string{"timestamp": "fixed"}, nil)
	if out2["timestamp"] != "fixed" {
		t.Errorf("file var should override built-in, got %q", out2["timestamp"])
	}
}
