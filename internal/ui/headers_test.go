package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMergeHeadersFormWins(t *testing.T) {
	base := map[string]string{"Authorization": "Bearer global", "X-Env": "prod"}
	form := map[string]string{"Authorization": "Bearer local", "X-Trace": "abc"}

	got := mergeHeaders(base, form)
	if got["Authorization"] != "Bearer local" {
		t.Errorf("form header should win, got %q", got["Authorization"])
	}
	if got["X-Env"] != "prod" {
		t.Errorf("base header should carry through, got %q", got["X-Env"])
	}
	if got["X-Trace"] != "abc" {
		t.Errorf("form-only header missing, got %q", got["X-Trace"])
	}
}

// A global header passed to New must be sent even when the form declares none.
func TestGlobalHeaderIsSent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	m := New(loadAPI(t), srv.URL, map[string]string{"Authorization": "Bearer global"}, nil)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // GET /pets

	// satisfy the required "status" field
	m.form.setValue("status", "sold")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected BatchMsg, got %T", cmd())
	}
	for _, c := range batch {
		if r, ok := c().(responseMsg); ok {
			m = send(m, r)
		}
	}

	if gotAuth != "Bearer global" {
		t.Errorf("global Authorization header not sent, server saw %q", gotAuth)
	}
}
