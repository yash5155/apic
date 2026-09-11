// Package history persists the last request sent to each endpoint so the user
// can reload it instead of retyping ids. Sensitive headers (auth, cookies) are
// never written to disk — see redact in the ui layer.
package history

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Entry is one saved request, keyed in the store by "METHOD PATH".
type Entry struct {
	Path    map[string]string `json:"path,omitempty"`
	Query   map[string]string `json:"query,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	SavedAt time.Time         `json:"saved_at"`
}

// Store is the whole history file.
type Store struct {
	Entries map[string]Entry `json:"entries"`
}

const maxFileBytes = 8 << 20 // 8 MiB safety cap

// Path returns the on-disk location, e.g. ~/.config/apic/history.json.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "apic", "history.json"), nil
}

// Load reads the store. A missing file yields an empty store, not an error.
func Load() (*Store, error) {
	s := &Store{Entries: map[string]Entry{}}

	path, err := Path()
	if err != nil {
		return s, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes))
	if err != nil {
		return s, err
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, s); err != nil {
		// Corrupt file: start fresh rather than failing the whole app.
		return &Store{Entries: map[string]Entry{}}, nil
	}
	if s.Entries == nil {
		s.Entries = map[string]Entry{}
	}
	return s, nil
}

// Put records an entry in memory. Call Save to persist.
func (s *Store) Put(key string, e Entry) {
	if s.Entries == nil {
		s.Entries = map[string]Entry{}
	}
	s.Entries[key] = e
}

// Get returns the saved entry for a key, if any.
func (s *Store) Get(key string) (Entry, bool) {
	e, ok := s.Entries[key]
	return e, ok
}

// Save writes the store atomically-ish with owner-only permissions, since
// entries may still contain non-secret but private values.
func (s *Store) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
