package ui

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// refreshResponse rebuilds the viewport contents from the current result,
// wrapping to the pane width so long lines never bleed past the border. The
// raw body is kept intact in m.result for saving; only the display is wrapped.
func (m *Model) refreshResponse() {
	if m.result == nil {
		return
	}

	var content string
	switch {
	case m.result.Err != nil:
		content = errStyle.Render("Request failed:\n\n" + m.result.Err.Error())
	case m.showHeaders:
		content = renderHeaders(m.result.Headers) + "\n" + m.result.Body
	case m.filterPath != "":
		// Filter the raw body; keep m.result.Body intact for save/curl.
		out, err := applyDotPath(m.result.Body, m.filterPath)
		if err != nil {
			content = errStyle.Render("filter: "+err.Error()) + "\n\n" + m.result.Body
		} else {
			content = out
		}
	default:
		content = m.result.Body
	}

	if w := m.response.Width; w > 0 {
		content = lipgloss.NewStyle().Width(w).Render(content)
	}
	m.response.SetContent(content)
}

// renderHeaders formats response headers in stable, canonical order.
func renderHeaders(h http.Header) string {
	if len(h) == 0 {
		return dimStyle.Render("(no headers)")
	}
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		b.WriteString(selStyle.Render(name) + ": " + strings.Join(h[name], ", ") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// mergeHeaders combines base headers (e.g. a global auth token) with per-form
// headers. Form values win, so a user can override a global on one endpoint.
func mergeHeaders(base, form map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(form))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range form {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

// saveResponse writes the full response body to a JSON file in the current
// directory and returns the filename.
func saveResponse(body string) (string, error) {
	return saveText("apic-response", "json", body)
}

// saveText writes content to "<prefix>.<ext>", picking the first name that
// does not already exist so successive saves never clobber each other.
func saveText(prefix, ext, content string) (string, error) {
	for i := 0; ; i++ {
		name := fmt.Sprintf("%s.%s", prefix, ext)
		if i > 0 {
			name = fmt.Sprintf("%s-%d.%s", prefix, i, ext)
		}
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return name, os.WriteFile(name, []byte(content), 0o644)
		}
	}
}
