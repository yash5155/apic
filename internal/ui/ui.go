// Package ui is the Bubble Tea layer.
//
// Two screens: a filterable endpoint list, and a detail screen with the
// generated form on the left and the response on the right.
package ui

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/yash5155/apic/internal/history"
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

	// response filter (dot-path, e.g. .data.items[0].name)
	filterInput textinput.Model
	queryActive bool
	filterPath  string

	// runtime server switching
	servers   []string
	serverIdx int

	// environments & variable interpolation
	envs       *EnvSet
	vars       map[string]string
	envHeaders map[string]string

	// request chaining: values captured from responses into {{vars}}
	captured      map[string]string
	captureInput  textinput.Model
	captureActive bool

	// request history
	store            *history.Store
	historyAvailable bool

	width, height int
	ready         bool
}

// New builds the initial model. baseHeaders are merged into every request the
// user sends (form values win on conflict); pass nil for none. envs carries the
// active environment and its variables; pass nil when no config is in play.
func New(api *spec.API, baseURL string, baseHeaders map[string]string, envs *EnvSet) Model {
	filter := textinput.New()
	filter.Placeholder = "filter endpoints"
	filter.Prompt = "/ "
	filter.Width = 30

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	query := textinput.New()
	query.Placeholder = ".data.items[0].name"
	query.Prompt = "filter "
	query.Width = 40

	capture := textinput.New()
	capture.Placeholder = "name = .data.token"
	capture.Prompt = "capture "
	capture.Width = 40

	store, _ := history.Load() // missing/unreadable history is non-fatal

	m := Model{
		api:          api,
		baseURL:      baseURL,
		baseHeaders:  baseHeaders,
		filter:       filter,
		filterInput:  query,
		captureInput: capture,
		spin:         sp,
		response:     viewport.New(40, 20),
		servers:      buildServerList(baseURL, api.Servers),
		store:        store,
		envs:         envs,
		vars:         envs.Vars(),
		envHeaders:   envs.Headers(),
	}
	m.applyFilter()
	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

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
		m.filterPath = ""
		m.refreshResponse()
		m.response.GotoTop()
		// Remember a successful request so it can be reloaded later.
		if res.Err == nil && res.Status >= 200 && res.Status < 300 {
			m.saveHistory()
		}
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

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While the response filter box is open, keys belong to it.
	if m.queryActive {
		switch msg.String() {
		case "esc":
			m.queryActive = false
			m.filterInput.Blur()
			m.filterPath = ""
			m.filterInput.SetValue("")
			m.refreshResponse()
			return m, nil
		case "enter":
			m.queryActive = false
			m.filterInput.Blur()
			m.filterPath = strings.TrimSpace(m.filterInput.Value())
			m.refreshResponse()
			m.response.GotoTop()
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	// While the capture box is open, keys belong to it.
	if m.captureActive {
		switch msg.String() {
		case "esc":
			m.captureActive = false
			m.captureInput.Blur()
			m.captureInput.SetValue("")
			return m, nil
		case "enter":
			m.captureActive = false
			m.captureInput.Blur()
			m.errMsg = m.doCapture(m.captureInput.Value())
			m.captureInput.SetValue("")
			return m, nil
		}
		var cmd tea.Cmd
		m.captureInput, cmd = m.captureInput.Update(msg)
		return m, cmd
	}

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

	// Cycle the base URL among the spec's servers.
	case "ctrl+e":
		if m.form.bodyFocused() {
			break // let the textarea use ctrl+e (end of line)
		}
		if len(m.servers) > 1 {
			m.serverIdx = (m.serverIdx + 1) % len(m.servers)
			m.baseURL = m.servers[m.serverIdx]
			m.errMsg = "server: " + m.baseURL
		}
		return m, nil

	// Cycle the active environment.
	case "ctrl+n":
		if m.form.bodyFocused() {
			break
		}
		if m.envs.HasMultiple() {
			m.cycleEnv()
		}
		return m, nil

	// Filter the response body with a dot-path query.
	case "ctrl+f":
		if m.form.bodyFocused() {
			break // let the textarea use ctrl+f (forward char)
		}
		if m.result == nil || m.result.Err != nil {
			m.errMsg = "no response to filter yet"
			return m, nil
		}
		m.queryActive = true
		m.filterInput.SetValue(m.filterPath)
		return m, m.filterInput.Focus()

	// Validate the JSON body against the schema on demand.
	case "ctrl+b":
		if m.form.bodyFocused() {
			break // let the textarea use ctrl+b (back char)
		}
		if v := m.form.endpoint.ValidateBody; v != nil {
			if msg := v(m.form.bodyValue()); msg != "" {
				m.errMsg = "body invalid: " + msg
			} else {
				m.errMsg = "body valid"
			}
		} else {
			m.errMsg = "this endpoint has no JSON body"
		}
		return m, nil

	// Copy the equivalent curl command to the clipboard.
	case "ctrl+y":
		req, missing := m.buildRequest()
		if len(missing) > 0 {
			m.errMsg = "unresolved variables: " + strings.Join(missing, ", ")
			return m, nil
		}
		cmd, err := buildCurl(req)
		if err != nil {
			m.errMsg = "curl failed: " + err.Error()
			return m, nil
		}
		if err := clipboard.WriteAll(cmd); err != nil {
			name, ferr := saveText("apic-curl", "sh", cmd)
			if ferr != nil {
				m.errMsg = "clipboard unavailable: " + err.Error()
			} else {
				m.errMsg = "clipboard unavailable, wrote " + name
			}
		} else {
			m.errMsg = "copied curl to clipboard"
		}
		return m, nil

	// Reload the last request sent to this endpoint.
	case "ctrl+p":
		if !m.historyAvailable {
			m.errMsg = "no saved request for this endpoint"
			return m, nil
		}
		m.reloadHistory()
		return m, nil

	// Capture a value from the response into a {{variable}} for later requests.
	case "ctrl+k":
		if m.form.bodyFocused() {
			break // let the textarea use ctrl+k (kill to end of line)
		}
		if m.result == nil || m.result.Err != nil {
			m.errMsg = "no response to capture from"
			return m, nil
		}
		m.captureActive = true
		if m.filterPath != "" {
			m.captureInput.SetValue("value = " + m.filterPath)
		}
		return m, m.captureInput.Focus()

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
		// Validate the JSON body against the schema before firing a request
		// that is otherwise guaranteed to 400.
		if v := m.form.endpoint.ValidateBody; v != nil {
			if msg := v(m.form.bodyValue()); msg != "" {
				m.errMsg = "body invalid: " + msg
				return m, nil
			}
		}
		m.errMsg = ""
		m.sending = true
		m.result = nil

		// Build + interpolate; refuse to send if any {{var}} is unresolved.
		req, missing := m.buildRequest()
		if len(missing) > 0 {
			m.errMsg = "unresolved variables: " + strings.Join(missing, ", ")
			m.sending = false
			return m, nil
		}

		// Two commands at once: start the spinner ticking AND fire the
		// request. tea.Batch runs them concurrently.
		return m, tea.Batch(m.spin.Tick, sendRequest(req))
	}

	var cmd tea.Cmd
	m.form, cmd = m.form.update(msg)
	return m, cmd
}
