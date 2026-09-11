package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// sendAndCapture drives: send GET /x, capture ".token" as {{token}}, and returns
// the model. The server returns {"token":"abc","id":7}.
func sendAndCapture(t *testing.T, m Model) Model {
	t.Helper()
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = send(m, ctrl(tea.KeyEnter)) // open GET /x

	// Fire the request and feed the response back.
	next, cmd := m.Update(ctrl(tea.KeyCtrlS))
	m = next.(Model)
	batch := cmd().(tea.BatchMsg)
	for _, c := range batch {
		if r, ok := c().(responseMsg); ok {
			m = send(m, r)
		}
	}
	if m.result == nil || m.result.Status != 200 {
		t.Fatalf("expected a 200 response, got %+v", m.result)
	}

	// ctrl+k opens the capture box; type "token = .token"; enter.
	m = send(m, ctrl(tea.KeyCtrlK))
	if !m.captureActive {
		t.Fatal("ctrl+k should open the capture box")
	}
	m = typeText(m, "token = .token")
	m = send(m, ctrl(tea.KeyEnter))
	return m
}

func TestCaptureAndReuse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"abc","id":7}`))
	}))
	defer srv.Close()

	m := New(oneEndpointAPI(), srv.URL, nil, NewEnvSet(twoEnvConfig(), "prod", nil, false))
	m = sendAndCapture(t, m)

	if got := m.captured["token"]; got != "abc" {
		t.Fatalf("captured token = %q, want abc", got)
	}

	// Reuse it: set the query field to {{token}} and confirm interpolation.
	m.form.setValue("id", "{{token}}")
	req, missing := m.buildRequest()
	if len(missing) != 0 {
		t.Fatalf("unexpected unresolved: %v", missing)
	}
	if req.Query["id"] != "abc" {
		t.Errorf("query id = %q, want captured value abc", req.Query["id"])
	}
}

func TestCapturedSurvivesEnvSwitch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"token":"abc","id":7}`))
	}))
	defer srv.Close()

	m := New(oneEndpointAPI(), srv.URL, nil, NewEnvSet(twoEnvConfig(), "prod", nil, false))
	m = sendAndCapture(t, m)

	m = send(m, ctrl(tea.KeyCtrlN)) // switch environment
	if m.effectiveVars()["token"] != "abc" {
		t.Errorf("captured var should survive env switch, got %q", m.effectiveVars()["token"])
	}
}

func TestCaptureRawScalarNoQuotes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"token":"abc","id":7}`))
	}))
	defer srv.Close()

	m := New(oneEndpointAPI(), srv.URL, nil, NewEnvSet(twoEnvConfig(), "prod", nil, false))
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = send(m, ctrl(tea.KeyEnter))
	next, cmd := m.Update(ctrl(tea.KeyCtrlS))
	m = next.(Model)
	for _, c := range cmd().(tea.BatchMsg) {
		if r, ok := c().(responseMsg); ok {
			m = send(m, r)
		}
	}

	// Capture an integer with only a name (path defaults to nothing -> id).
	m = send(m, ctrl(tea.KeyCtrlK))
	m = typeText(m, "n = .id")
	m = send(m, ctrl(tea.KeyEnter))
	if m.captured["n"] != "7" { // not "7.0", not quoted
		t.Errorf("captured id = %q, want 7", m.captured["n"])
	}
}
