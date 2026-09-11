// Package ui is the Bubble Tea layer.
//
// Two screens: a filterable endpoint list, and a detail screen with the
// generated form on the left and the response on the right.
package ui

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/yash5155/apic/internal/httpx"
	"github.com/yash5155/apic/internal/spec"
)

type screen int

const (
	screenList screen = iota
	screenDetail
)

// responseMsg carries a finished request back into Update.
//
// This is the pattern that makes async work in Bubble Tea: the request
// runs in a goroutine, and when it finishes it delivers a message just
// like a keypress. Update never blocks, so the UI never freezes.
type responseMsg httpx.Result

// sendRequest returns a command. Bubble Tea runs the inner function on its
// own goroutine and feeds whatever it returns back into Update.
func sendRequest(r httpx.Request) tea.Cmd {
	return func() tea.Msg {
		return responseMsg(httpx.Send(r))
	}
}

// Model is the whole application state.
type Model struct {
	api         *spec.API
	baseURL     string
	baseHeaders map[string]string // applied to every request (e.g. auth)

	screen screen

	// list screen
	visible   []int // indexes into api.Endpoints that survive the filter
	cursor    int
	offset    int // first visible row, for scrolling
	filter    textinput.Model
	filtering bool

	// detail screen
	form        form
	response    viewport.Model
	result      *httpx.Result
	sending     bool
	spin        spinner.Model
	errMsg      string
	showHeaders bool // toggle response headers view

	width, height int
	ready         bool
}

// New builds the initial model. baseHeaders are merged into every request the
// user sends (form values win on conflict); pass nil for none.
func New(api *spec.API, baseURL string, baseHeaders map[string]string) Model {
	filter := textinput.New()
	filter.Placeholder = "filter endpoints"
	filter.Prompt = "/ "
	filter.Width = 30

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := Model{
		api:         api,
		baseURL:     baseURL,
		baseHeaders: baseHeaders,
		filter:      filter,
		spin:        sp,
		response:    viewport.New(40, 20),
	}
	m.applyFilter()
	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		// The viewport must be sized to the pane's inner text area, or long
		// lines spill past the border. paneWidth() is the shared source of
		// truth; subtract the border (2) and padding (2) it adds.
		m.response.Width = max(10, m.paneWidth()-4)
		// Leave room inside the pane for its title and status lines.
		m.response.Height = max(3, msg.Height-12)
		m.refreshResponse()
		return m, nil

	case spinner.TickMsg:
		if m.sending {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case responseMsg:
		res := httpx.Result(msg)
		m.sending = false
		m.result = &res
		m.showHeaders = false
		m.refreshResponse()
		m.response.GotoTop()
		return m, nil

	case tea.MouseMsg:
		// Mouse wheel scrolls the response pane on the detail screen.
		if m.screen == screenDetail {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.response.LineUp(3)
			case tea.MouseButtonWheelDown:
				m.response.LineDown(3)
			}
		}
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.screen == screenList {
			return m.updateList(msg)
		}
		return m.updateDetail(msg)
	}

	return m, nil
}

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
		m.form = newForm(ep)
		m.result = nil
		m.errMsg = ""
		m.response.SetContent(dimStyle.Render("Press ctrl+s to send the request."))
		m.screen = screenDetail
		return m, textinput.Blink
	}

	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenList
		return m, nil

	case "tab":
		m.form.next()
		return m, nil

	case "shift+tab":
		m.form.prev()
		return m, nil

	// Scroll the response pane without stealing keys from the form.
	// ctrl+d / ctrl+u work on every keyboard (no PgDn key needed).
	case "pgdown", "ctrl+d":
		m.response.HalfViewDown()
		return m, nil
	case "pgup", "ctrl+u":
		m.response.HalfViewUp()
		return m, nil
	case "ctrl+g":
		m.response.GotoBottom()
		return m, nil
	case "ctrl+t":
		m.response.GotoTop()
		return m, nil

	// Toggle the response headers view.
	case "ctrl+r":
		if m.result != nil && m.result.Err == nil {
			m.showHeaders = !m.showHeaders
			m.refreshResponse()
			m.response.GotoTop()
		}
		return m, nil

	// Save the full response body to a file so it can be opened elsewhere.
	case "ctrl+o":
		if m.result == nil {
			m.errMsg = "no response to save yet"
			return m, nil
		}
		name, err := saveResponse(m.result.Body)
		if err != nil {
			m.errMsg = "save failed: " + err.Error()
		} else {
			m.errMsg = "saved full response to " + name
		}
		return m, nil

	case "ctrl+s":
		if m.sending {
			return m, nil
		}
		if missing := m.form.missingRequired(); len(missing) > 0 {
			m.errMsg = "missing required: " + strings.Join(missing, ", ")
			return m, nil
		}
		m.errMsg = ""
		m.sending = true
		m.result = nil

		path, query, headers := m.form.values()
		req := httpx.Request{
			Method:     m.form.endpoint.Method,
			BaseURL:    m.baseURL,
			Path:       m.form.endpoint.Path,
			PathParams: path,
			Query:      query,
			Headers:    mergeHeaders(m.baseHeaders, headers),
			Body:       m.form.bodyValue(),
		}

		// Two commands at once: start the spinner ticking AND fire the
		// request. tea.Batch runs them concurrently.
		return m, tea.Batch(m.spin.Tick, sendRequest(req))
	}

	var cmd tea.Cmd
	m.form, cmd = m.form.update(msg)
	return m, cmd
}

// ---------------------------------------------------------------- view

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
	b.WriteString("\n" + helpStyle.Render("j/k move · / filter · enter open · q quit"))
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
		right.WriteString("\n\n")
	case m.result != nil:
		right.WriteString(errStyle.Render("error") + "\n\n")
	default:
		right.WriteString("\n\n")
	}
	right.WriteString(m.response.View())

	// Both panes get the same explicit height, otherwise the shorter one's
	// border stops early and the layout looks broken.
	paneH := max(8, m.height-8)
	paneW := m.paneWidth()

	leftPane := paneStyle.Width(paneW).Height(paneH).Render(left)
	rightPane := paneStyle.Width(paneW).Height(paneH).Render(right.String())

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	footer := helpStyle.Render("tab field · ctrl+s send · ctrl+d/u scroll · ctrl+r headers · ctrl+o save · esc back")
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

func (m Model) header() string {
	title := m.api.Title
	if title == "" {
		title = "API"
	}
	return titleStyle.Render(title) + dimStyle.Render("  "+m.baseURL)
}

// ---------------------------------------------------------------- helpers

// saveResponse writes the full response body to a file in the current
// directory and returns the filename. It picks the first name that does
// not already exist so successive saves do not clobber each other.
func saveResponse(body string) (string, error) {
	for i := 0; ; i++ {
		name := "apic-response.json"
		if i > 0 {
			name = fmt.Sprintf("apic-response-%d.json", i)
		}
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return name, os.WriteFile(name, []byte(body), 0o644)
		}
	}
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
