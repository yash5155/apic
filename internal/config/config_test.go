package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Environments) != 0 {
		t.Fatalf("fresh config should be empty, got %d", len(c.Environments))
	}

	c.Active = "prod"
	c.Environments["prod"] = Environment{
		BaseURL: "https://api.example.com",
		Headers: map[string]string{"X-Env": "prod"},
		Vars:    map[string]string{"token": "abc"},
	}
	c.Environments["staging"] = Environment{BaseURL: "https://staging.example.com"}
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	c2, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if c2.Active != "prod" {
		t.Errorf("active = %q", c2.Active)
	}
	env, ok := c2.Env("prod")
	if !ok || env.BaseURL != "https://api.example.com" || env.Vars["token"] != "abc" {
		t.Errorf("prod env = %+v (ok=%v)", env, ok)
	}
	if got := c2.Names(); len(got) != 2 || got[0] != "prod" || got[1] != "staging" {
		t.Errorf("Names() = %v, want sorted [prod staging]", got)
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatalf("Load on missing should not error: %v", err)
	}
	if _, ok := c.Env("nope"); ok {
		t.Error("missing config should have no envs")
	}
}

func TestSaveIsOwnerOnly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c := &Config{Environments: map[string]Environment{"x": {}}}
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, _ := Path()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perms = %o, want 600", perm)
	}
}

func TestCorruptFileStartsFresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "apic", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("corrupt file should not error: %v", err)
	}
	if len(c.Environments) != 0 {
		t.Errorf("corrupt file should yield empty config, got %+v", c.Environments)
	}
}
