package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While filtering, most keys belong to the text input.
	if m.filtering {
		switch msg.String() {
		case "esc":
			m.filtering = false
			m.filter.Blur()
			m.filter.SetValue("")
			m.applyFilter()
			return m, nil
		case "enter":
			m.filtering = false
			m.filter.Blur()
			return m, nil
		}

		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.applyFilter()
		return m, cmd
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "ctrl+e":
		if len(m.servers) > 1 {
			m.serverIdx = (m.serverIdx + 1) % len(m.servers)
			m.baseURL = m.servers[m.serverIdx]
		}
		return m, nil
	case "ctrl+n":
		if m.envs.HasMultiple() {
			m.cycleEnv()
		}
		return m, nil
	case "/":
		m.filtering = true
		return m, m.filter.Focus()
	case "esc":
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.applyFilter()
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.clampScroll()
		}
	case "down", "j":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
			m.clampScroll()
		}
	case "enter":
		if len(m.visible) == 0 {
			return m, nil
		}
		ep := m.api.Endpoints[m.visible[m.cursor]]
		m.form = newForm(ep, m.api.Security, m.baseHeaders)
		m.result = nil
		m.errMsg = ""
		m.filterPath = ""
		m.queryActive = false
		_, m.historyAvailable = m.store.Get(historyKey(ep))
		m.response.SetContent(dimStyle.Render("Press ctrl+s to send the request."))
		m.screen = screenDetail
		return m, textinput.Blink
	}

	return m, nil
}

// buildServerList puts the active base URL first, then any other servers the
// spec declares, de-duplicated — the order the ctrl+e switcher cycles through.
func buildServerList(baseURL string, servers []string) []string {
	out := []string{baseURL}
	seen := map[string]bool{baseURL: true}
	for _, s := range servers {
		if s != "" && !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

// applyFilter recomputes which endpoints are shown. Matching is a simple
// case-insensitive substring test against "METHOD /path summary".
func (m *Model) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	m.visible = m.visible[:0]

	for i, ep := range m.api.Endpoints {
		if q == "" {
			m.visible = append(m.visible, i)
			continue
		}
		hay := strings.ToLower(ep.Method + " " + ep.Path + " " + ep.Summary)
		if strings.Contains(hay, q) {
			m.visible = append(m.visible, i)
		}
	}

	if m.cursor >= len(m.visible) {
		m.cursor = max(0, len(m.visible)-1)
	}
	m.offset = 0
}

// listHeight is how many endpoint rows fit on screen.
func (m Model) listHeight() int {
	h := m.height - 8
	if h < 3 {
		return 3
	}
	return h
}

// clampScroll keeps the cursor inside the visible window.
func (m *Model) clampScroll() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
}
