package ui

import "strings"

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
