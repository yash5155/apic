package history

import (
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	// Sandbox the config dir so we don't touch the real one.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Entries) != 0 {
		t.Fatalf("fresh store should be empty, got %d", len(s.Entries))
	}

	s.Put("GET /pets", Entry{
		Query:   map[string]string{"status": "sold"},
		Body:    `{"a":1}`,
		SavedAt: time.Unix(1000, 0),
	})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload from disk and confirm persistence.
	s2, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	e, ok := s2.Get("GET /pets")
	if !ok {
		t.Fatal("entry not persisted")
	}
	if e.Query["status"] != "sold" || e.Body != `{"a":1}` {
		t.Errorf("entry = %+v", e)
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}
	if _, ok := s.Get("nope"); ok {
		t.Error("missing store should have no entries")
	}
}
