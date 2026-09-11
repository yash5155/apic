package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yash5155/apic/internal/spec"
)

func loadAPI(t *testing.T) *spec.API {
	t.Helper()
	api, err := spec.Load("../../testdata/petstore.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return api
}

func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func send(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func typeText(m Model, s string) Model {
	for _, r := range s {
		m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func sized(t *testing.T, base string) Model {
	m := New(loadAPI(t), base, nil)
	return send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
}

func TestFilterNarrowsTheList(t *testing.T) {
	m := sized(t, "https://x.test")
	if len(m.visible) != 4 {
		t.Fatalf("expected 4 visible, got %d", len(m.visible))
	}

	m = send(m, runes("/"))
	m = typeText(m, "petId")
	if len(m.visible) != 2 {
		t.Errorf("expected 2 matches for petId, got %d", len(m.visible))
	}

	// esc clears the filter and restores everything
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.visible) != 4 {
		t.Errorf("filter not cleared, got %d", len(m.visible))
	}
}

func TestFilterMatchesMethodAndSummary(t *testing.T) {
	m := sized(t, "https://x.test")
	m = send(m, runes("/"))
	m = typeText(m, "delete")
	if len(m.visible) != 1 {
		t.Fatalf("expected 1 match, got %d", len(m.visible))
	}
	if ep := m.api.Endpoints[m.visible[0]]; ep.Method != "DELETE" {
		t.Errorf("matched %s", ep.Method)
	}
}

// Opening GET /pets must build one input per parameter, including the
// path-level header param.
func TestFormIsGeneratedFromSchema(t *testing.T) {
	m := sized(t, "https://x.test")
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // first row = GET /pets

	if m.screen != screenDetail {
		t.Fatal("should be on the detail screen")
	}
	if got := len(m.form.inputs); got != 3 {
		t.Fatalf("expected 3 inputs, got %d", got)
	}
	if m.form.hasBody {
		t.Error("GET should not have a body pane")
	}

	// the integer param picked up its default
	var limitValue string
	for i, p := range m.form.endpoint.Params {
		if p.Name == "limit" {
			limitValue = m.form.inputs[i].Value()
		}
	}
	if limitValue != "20" {
		t.Errorf("limit should be prefilled with its default, got %q", limitValue)
	}

	// the enum became a placeholder hint
	view := m.form.view()
	if !strings.Contains(view, "available | pending | sold") {
		t.Error("enum values should show as a placeholder hint")
	}
}

func TestPostEndpointGetsBodyPane(t *testing.T) {
	m := sized(t, "https://x.test")
	m = send(m, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter}) // POST /pets

	if !m.form.hasBody {
		t.Fatal("POST should have a body pane")
	}
	if !strings.Contains(m.form.body.Value(), "\"name\"") {
		t.Errorf("body not prefilled from schema: %q", m.form.body.Value())
	}
	// 1 header param + the body
	if m.form.fieldCount() != 2 {
		t.Errorf("fieldCount = %d", m.form.fieldCount())
	}
}

func TestTabCyclesFocusAndWraps(t *testing.T) {
	m := sized(t, "https://x.test")
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})

	n := m.form.fieldCount()
	for i := 0; i < n; i++ {
		if m.form.focus != i {
			t.Fatalf("focus = %d, want %d", m.form.focus, i)
		}
		m = send(m, tea.KeyMsg{Type: tea.KeyTab})
	}
	if m.form.focus != 0 {
		t.Errorf("focus should wrap to 0, got %d", m.form.focus)
	}

	m = send(m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.form.focus != n-1 {
		t.Errorf("shift+tab should wrap backwards, got %d", m.form.focus)
	}
}

func TestSendBlockedWhenRequiredFieldEmpty(t *testing.T) {
	m := sized(t, "https://x.test")
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // GET /pets, status is required
	m = send(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.sending {
		t.Error("should not have sent with a required field blank")
	}
	if !strings.Contains(m.errMsg, "status") {
		t.Errorf("errMsg = %q", m.errMsg)
	}
}

func TestValuesSplitByLocation(t *testing.T) {
	m := sized(t, "https://x.test")
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})

	for i, p := range m.form.endpoint.Params {
		switch p.Name {
		case "status":
			m.form.inputs[i].SetValue("available")
		case "X-Request-Id":
			m.form.inputs[i].SetValue("req-1")
		}
	}

	path, query, headers := m.form.values()
	if len(path) != 0 {
		t.Errorf("no path params expected, got %v", path)
	}
	if query["status"] != "available" || query["limit"] != "20" {
		t.Errorf("query = %v", query)
	}
	if headers["X-Request-Id"] != "req-1" {
		t.Errorf("headers = %v", headers)
	}
}

// The full loop: press ctrl+s, run the command Bubble Tea would run, feed
// the resulting message back into Update, and check the response rendered.
func TestFullRequestRoundTrip(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"pets":[{"id":1,"name":"Rex"}]}`))
	}))
	defer srv.Close()

	m := sized(t, srv.URL)
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})

	for i, p := range m.form.endpoint.Params {
		if p.Name == "status" {
			m.form.inputs[i].SetValue("sold")
		}
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if !m.sending {
		t.Fatal("should be in the sending state")
	}
	if cmd == nil {
		t.Fatal("ctrl+s must return a command")
	}

	// tea.Batch returns a BatchMsg holding the individual commands.
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected a BatchMsg, got %T", cmd())
	}

	var got *responseMsg
	for _, c := range batch {
		if r, ok := c().(responseMsg); ok {
			got = &r
		}
	}
	if got == nil {
		t.Fatal("no responseMsg came back from the batch")
	}

	m = send(m, *got)

	if m.sending {
		t.Error("sending flag should be cleared")
	}
	if m.result == nil || m.result.Status != 200 {
		t.Fatalf("result = %+v", m.result)
	}
	if gotPath != "/pets" || gotQuery != "limit=20&status=sold" {
		t.Errorf("server saw %s?%s", gotPath, gotQuery)
	}
	if !strings.Contains(m.result.Body, "Rex") {
		t.Errorf("body = %q", m.result.Body)
	}

	// and it should actually appear on screen
	if !strings.Contains(m.View(), "200 OK") {
		t.Error("status line missing from the detail view")
	}
}

func TestEscReturnsToList(t *testing.T) {
	m := sized(t, "https://x.test")
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenDetail {
		t.Fatal("expected detail screen")
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenList {
		t.Error("esc should go back to the list")
	}
}

func TestListViewRenders(t *testing.T) {
	m := sized(t, "https://x.test")
	view := m.View()
	for _, want := range []string{"Petstore", "GET", "/pets/{petId}", "4/4 endpoints"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}
