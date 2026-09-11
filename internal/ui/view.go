package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/yash5155/apic/internal/httpx"
)

func (m Model) View() string {
	if !m.ready {
		return "loading..."
	}
	if m.screen == screenList {
		return m.listView()
	}
	return m.detailView()
}

func (m Model) listView() string {
	var b strings.Builder
	b.WriteString(m.header() + "\n\n")

	if m.filtering || m.filter.Value() != "" {
		b.WriteString(m.filter.View() + "\n\n")
	}

	if len(m.visible) == 0 {
		b.WriteString(dimStyle.Render("No endpoints match.") + "\n")
	}

	h := m.listHeight()
	end := min(m.offset+h, len(m.visible))

	for i := m.offset; i < end; i++ {
		ep := m.api.Endpoints[m.visible[i]]
		row := methodStyle(ep.Method).Render(fmt.Sprintf(" %-6s ", ep.Method)) + " "

		path := ep.Path
		if i == m.cursor {
			row += selStyle.Render(path)
		} else {
			row += pathStyle.Render(path)
		}
		if ep.Summary != "" {
			row += dimStyle.Render("  " + truncate(ep.Summary, 40))
		}

		if i == m.cursor {
			b.WriteString(cursorStyle.Render("›") + " " + row + "\n")
		} else {
			b.WriteString("  " + row + "\n")
		}
	}

	b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("%d/%d endpoints", len(m.visible), len(m.api.Endpoints))))
	hints := "j/k move · / filter · ctrl+e server · enter open · q quit"
	if m.envs.HasMultiple() {
		hints = "j/k move · / filter · ctrl+e server · ctrl+n env · enter open · q quit"
	}
	b.WriteString("\n" + helpStyle.Render(hints))
	return b.String()
}

func (m Model) detailView() string {
	left := m.form.view()

	var right strings.Builder
	right.WriteString(titleStyle.Render("Response") + "\n")

	switch {
	case m.sending:
		right.WriteString(m.spin.View() + " sending...\n\n")
	case m.result != nil && m.result.Err == nil:
		right.WriteString(statusStyle(m.result.Status).Render(httpx.StatusText(m.result.Status)))
		right.WriteString(dimStyle.Render(fmt.Sprintf("  %s", m.result.Duration.Round(1e6))))
		if m.result.Truncated {
			right.WriteString(errStyle.Render("  (truncated — ctrl+o to save full)"))
		}
		if m.showHeaders {
			right.WriteString(dimStyle.Render("  [headers]"))
		}
		if m.filterPath != "" {
			right.WriteString(dimStyle.Render("  [filter " + m.filterPath + "]"))
		}
		right.WriteString("\n\n")
	case m.result != nil:
		right.WriteString(errStyle.Render("error") + "\n\n")
	default:
		right.WriteString("\n\n")
	}
	if m.queryActive {
		right.WriteString(m.filterInput.View() + "\n\n")
	}
	if m.captureActive {
		right.WriteString(m.captureInput.View() + "\n\n")
	}
	right.WriteString(m.response.View())

	// Both panes get the same explicit height, otherwise the shorter one's
	// border stops early and the layout looks broken.
	paneH := max(8, m.height-8)
	paneW := m.paneWidth()

	leftPane := paneStyle.Width(paneW).Height(paneH).Render(left)
	rightPane := paneStyle.Width(paneW).Height(paneH).Render(right.String())

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	line2 := "ctrl+r headers · ctrl+d/u scroll · ctrl+e server · ctrl+p reload · esc back"
	if m.envs.HasMultiple() {
		line2 = "ctrl+r headers · ctrl+d/u scroll · ctrl+e server · ctrl+n env · ctrl+p reload · esc back"
	}
	hints := "tab field · ‹›enum · ctrl+s send · ctrl+b validate · ctrl+y curl · ctrl+f filter · ctrl+k capture · ctrl+o save\n" + line2
	footer := helpStyle.Render(hints)
	if m.errMsg != "" {
		footer = errStyle.Render(m.errMsg) + "\n" + footer
	}

	return m.header() + "\n\n" + body + "\n" + footer
}

// paneWidth is the width of each of the two side-by-side panes, chosen so both
// fit within the terminal once lipgloss adds their borders. It is the single
// source of truth shared by detailView and the viewport sizing in Update.
func (m Model) paneWidth() int {
	return max(30, m.width/2-2)
}

func (m Model) header() string {
	title := m.api.Title
	if title == "" {
		title = "API"
	}
	h := titleStyle.Render(title) + dimStyle.Render("  "+m.baseURL)
	if name := m.envs.ActiveName(); name != "" {
		h += "  " + selStyle.Render("["+name+"]")
	}
	if n := len(m.captured); n > 0 {
		h += "  " + dimStyle.Render(fmt.Sprintf("{%d captured}", n))
	}
	return h
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
