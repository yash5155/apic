package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yash5155/apic/internal/config"
	"github.com/yash5155/apic/internal/spec"
)

func twoEnvConfig() *config.Config {
	return &config.Config{
		Active: "prod",
		Environments: map[string]config.Environment{
			"prod":    {BaseURL: "https://a.test", Vars: map[string]string{"id": "7"}},
			"staging": {BaseURL: "https://b.test"},
		},
	}
}

func oneEndpointAPI() *spec.API {
	return &spec.API{
		Title:     "T",
		Endpoints: []spec.Endpoint{{Method: "GET", Path: "/x", Params: []spec.Param{{Name: "id", In: "query"}}}},
	}
}

func TestEnvCycleUpdatesBaseAndHeader(t *testing.T) {
	envs := NewEnvSet(twoEnvConfig(), "prod", nil, false)
	m := New(oneEndpointAPI(), "https://a.test", nil, envs)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if !strings.Contains(m.View(), "[prod]") {
		t.Errorf("header should show [prod]:\n%s", m.View())
	}
	m = send(m, ctrl(tea.KeyCtrlN))
	if m.envs.ActiveName() != "staging" {
		t.Errorf("active env = %q, want staging", m.envs.ActiveName())
	}
	if m.baseURL != "https://b.test" {
		t.Errorf("baseURL after switch = %q, want b.test", m.baseURL)
	}
	if !strings.Contains(m.View(), "[staging]") {
		t.Error("header should show [staging] after switch")
	}
}

func TestServerForcedIgnoresEnvBase(t *testing.T) {
	envs := NewEnvSet(twoEnvConfig(), "prod", nil, true) // --server was given
	m := New(oneEndpointAPI(), "https://forced.test", nil, envs)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m = send(m, ctrl(tea.KeyCtrlN))
	if m.baseURL != "https://forced.test" {
		t.Errorf("serverForced base changed to %q", m.baseURL)
	}
}

func TestInterpolationFillsRequest(t *testing.T) {
	envs := NewEnvSet(twoEnvConfig(), "prod", nil, false)
	m := New(oneEndpointAPI(), "https://a.test", nil, envs)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = send(m, ctrl(tea.KeyEnter)) // open GET /x

	m.form.setValue("id", "{{id}}")
	req, missing := m.buildRequest()
	if len(missing) != 0 {
		t.Fatalf("unexpected unresolved: %v", missing)
	}
	if req.Query["id"] != "7" {
		t.Errorf("query id = %q, want interpolated 7", req.Query["id"])
	}
}

func TestUnresolvedVarBlocksSend(t *testing.T) {
	envs := NewEnvSet(twoEnvConfig(), "prod", nil, false)
	m := New(oneEndpointAPI(), "https://a.test", nil, envs)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = send(m, ctrl(tea.KeyEnter))

	m.form.setValue("id", "{{token}}") // no such var
	m = send(m, ctrl(tea.KeyCtrlS))

	if m.sending {
		t.Error("should not send with an unresolved variable")
	}
	if !strings.Contains(m.errMsg, "unresolved variables: token") {
		t.Errorf("errMsg = %q", m.errMsg)
	}
}
