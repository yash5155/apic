// Package config persists named environments (prod/staging/local) and the
// variables/headers that go with them, so apic works like a reusable workspace
// instead of a one-shot client. It is intentionally pure: no kin-openapi and no
// Bubble Tea, so it can be reused and tested in isolation.
package config

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Environment is one named target: an optional base URL, default headers, and
// variables usable as {{name}} anywhere in a request.
type Environment struct {
	BaseURL string            `json:"base_url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Vars    map[string]string `json:"vars,omitempty"`
}

// Config is the whole ~/.config/apic/config.json file.
type Config struct {
	Active       string                 `json:"active,omitempty"`
	Environments map[string]Environment `json:"environments,omitempty"`
}

const maxFileBytes = 1 << 20 // 1 MiB safety cap

// Path returns the on-disk location, e.g. ~/.config/apic/config.json.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "apic", "config.json"), nil
}

// Load reads the config. A missing or corrupt file yields an empty config (never
// nil), not an error, so callers never have to nil-check.
func Load() (*Config, error) {
	c := &Config{Environments: map[string]Environment{}}

	path, err := Path()
	if err != nil {
		return c, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes))
	if err != nil {
		return c, err
	}
	if len(data) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(data, c); err != nil {
		return &Config{Environments: map[string]Environment{}}, nil // corrupt: start fresh
	}
	if c.Environments == nil {
		c.Environments = map[string]Environment{}
	}
	return c, nil
}

// Save writes the config with owner-only permissions, since it may hold tokens.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Env returns the named environment, if present.
func (c *Config) Env(name string) (Environment, bool) {
	e, ok := c.Environments[name]
	return e, ok
}

// Names returns the environment names in sorted order, so the switcher and
// error messages are deterministic.
func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Environments))
	for name := range c.Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
