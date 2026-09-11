package ui

import (
	"strings"
	"time"

	"github.com/yash5155/apic/internal/history"
	"github.com/yash5155/apic/internal/spec"
)

// historyKey identifies an endpoint in the history store.
func historyKey(ep spec.Endpoint) string { return ep.Method + " " + ep.Path }

// saveHistory records the current request, stripping secrets first.
func (m *Model) saveHistory() {
	if m.store == nil {
		return
	}
	path, query, headers := m.form.values()
	sensitive := m.sensitiveNames()
	m.store.Put(historyKey(m.form.endpoint), history.Entry{
		Path:    path,
		Query:   redactMap(query, sensitive),
		Headers: redactMap(headers, sensitive),
		Body:    m.form.bodyValue(),
		SavedAt: time.Now(),
	})
	if err := m.store.Save(); err != nil {
		m.errMsg = "history save failed: " + err.Error()
		return
	}
	m.historyAvailable = true
}

// reloadHistory repopulates the form from the last saved request.
func (m *Model) reloadHistory() {
	e, ok := m.store.Get(historyKey(m.form.endpoint))
	if !ok {
		m.errMsg = "no saved request for this endpoint"
		return
	}
	for k, v := range e.Path {
		m.form.setValue(k, v)
	}
	for k, v := range e.Query {
		m.form.setValue(k, v)
	}
	for k, v := range e.Headers {
		m.form.setValue(k, v)
	}
	if e.Body != "" && m.form.hasBody {
		m.form.body.SetValue(e.Body)
	}
	m.errMsg = "reloaded last request (secrets were not stored)"
}

// sensitiveNames is the set of header/query names never written to history:
// Authorization, Cookie, and any apiKey scheme this endpoint uses.
func (m Model) sensitiveNames() map[string]bool {
	s := map[string]bool{"authorization": true, "cookie": true}
	for _, key := range m.form.endpoint.Auth {
		if sc, ok := m.api.Security[key]; ok && sc.Type == "apiKey" && sc.Name != "" {
			s[strings.ToLower(sc.Name)] = true
		}
	}
	return s
}

// redactMap drops entries whose (case-insensitive) key is sensitive.
func redactMap(in map[string]string, sensitive map[string]bool) map[string]string {
	if len(in) == 0 {
		return in
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if sensitive[strings.ToLower(k)] {
			continue
		}
		out[k] = v
	}
	return out
}
