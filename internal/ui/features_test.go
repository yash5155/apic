package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yash5155/apic/internal/spec"
)

func loadAPIFrom(t *testing.T, path string) *spec.API {
	t.Helper()
	api, err := spec.Load(path)
	if err != nil {
		t.Fatalf("Load %s: %v", path, err)
	}
	return api
}

func ctrl(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func TestServerSwitch(t *testing.T) {
	api := &spec.API{
		Title:     "T",
		Servers:   []string{"https://a.test", "https://b.test"},
		Endpoints: []spec.Endpoint{{Method: "GET", Path: "/x"}},
	}
	m := New(api, "https://a.test", nil, nil)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if len(m.servers) != 2 {
		t.Fatalf("servers = %v", m.servers)
	}
	m = send(m, ctrl(tea.KeyCtrlE))
	if m.baseURL != "https://b.test" {
		t.Errorf("after ctrl+e baseURL = %q, want b.test", m.baseURL)
	}
	m = send(m, ctrl(tea.KeyCtrlE))
	if m.baseURL != "https://a.test" {
		t.Errorf("ctrl+e should wrap to a.test, got %q", m.baseURL)
	}
}

func TestSecurityAutoFill(t *testing.T) {
	api := loadAPIFrom(t, "../../testdata/secured.yaml")
	m := New(api, "https://secure.example.test", map[string]string{"Authorization": "Bearer T"}, nil)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// GET /widgets (first, ApiKeyAuth) should synthesize the X-API-Key header.
	m = send(m, ctrl(tea.KeyEnter))
	if !m.form.hasParam("X-API-Key", "header") {
		t.Error("GET /widgets should have a synthesized X-API-Key field")
	}
	m = send(m, ctrl(tea.KeyEsc))

	// POST /widgets (BearerAuth) should get an Authorization field prefilled
	// from the global -H token.
	m = send(m, ctrl(tea.KeyDown), ctrl(tea.KeyEnter))
	if got := m.form.value("Authorization"); got != "Bearer T" {
		t.Errorf("Authorization = %q, want prefilled 'Bearer T'", got)
	}
}

func TestBodyValidationBlocksSend(t *testing.T) {
	api := loadAPIFrom(t, "../../testdata/secured.yaml")
	m := New(api, "https://secure.example.test", map[string]string{"Authorization": "Bearer T"}, nil)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m = send(m, ctrl(tea.KeyDown), ctrl(tea.KeyEnter)) // POST /widgets
	m.form.body.SetValue(`{"size": 3}`)                // missing required "name"

	m = send(m, ctrl(tea.KeyCtrlS))
	if m.sending {
		t.Error("send should be blocked by invalid body")
	}
	if !strings.Contains(m.errMsg, "body invalid") {
		t.Errorf("errMsg = %q, want a body-invalid message", m.errMsg)
	}
}

func TestHistorySavesAndRedacts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	api := loadAPIFrom(t, "../../testdata/secured.yaml")
	m := New(api, srv.URL, nil, nil)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// GET /widgets has a synthesized X-API-Key (sensitive) field.
	m = send(m, ctrl(tea.KeyEnter))
	m.form.setValue("X-API-Key", "super-secret")

	// Fire the request and feed the response back.
	next, cmd := m.Update(ctrl(tea.KeyCtrlS))
	m = next.(Model)
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected BatchMsg, got %T", cmd())
	}
	for _, c := range batch {
		if r, ok := c().(responseMsg); ok {
			m = send(m, r)
		}
	}

	e, ok := m.store.Get("GET /widgets")
	if !ok {
		t.Fatal("history not saved after a 200")
	}
	if _, leaked := e.Headers["X-API-Key"]; leaked {
		t.Errorf("sensitive apiKey header must not be stored: %v", e.Headers)
	}
}
