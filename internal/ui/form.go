package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/yash5155/apic/internal/spec"
)

// form is built at runtime from an endpoint's parameters.
//
// This is the real difference from a hand-written TUI: you don't know how
// many inputs there are until the user picks an endpoint, so the inputs
// live in a slice and focus is just an index into it.
type form struct {
	endpoint spec.Endpoint
	inputs   []textinput.Model
	body     textarea.Model
	hasBody  bool
	focus    int // 0..len(inputs)-1 = a param, len(inputs) = the body
}

func newForm(ep spec.Endpoint) form {
	f := form{
		endpoint: ep,
		hasBody:  ep.BodySkeleton != "",
	}

	for _, p := range ep.Params {
		in := textinput.New()
		in.Placeholder = placeholderFor(p)
		in.Width = 34
		in.CharLimit = 256
		if p.Default != "" {
			in.SetValue(p.Default)
		}
		f.inputs = append(f.inputs, in)
	}

	if f.hasBody {
		ta := textarea.New()
		ta.SetValue(ep.BodySkeleton)
		ta.SetWidth(38)
		ta.SetHeight(8)
		ta.ShowLineNumbers = false
		f.body = ta
	}

	f.focusCurrent()
	return f
}

// placeholderFor turns schema info into a useful hint.
func placeholderFor(p spec.Param) string {
	if len(p.Enum) > 0 {
		return strings.Join(p.Enum, " | ")
	}
	return p.Type
}

// fieldCount includes the body pane if there is one.
func (f form) fieldCount() int {
	if f.hasBody {
		return len(f.inputs) + 1
	}
	return len(f.inputs)
}

// focusCurrent blurs everything, then focuses whatever focus points at.
// Calling this after every focus change is simpler than tracking which
// field used to be focused.
func (f *form) focusCurrent() {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	if f.hasBody {
		f.body.Blur()
	}

	if f.focus < len(f.inputs) {
		f.inputs[f.focus].Focus()
	} else if f.hasBody {
		f.body.Focus()
	}
}

func (f *form) next() {
	if f.fieldCount() == 0 {
		return
	}
	f.focus = (f.focus + 1) % f.fieldCount()
	f.focusCurrent()
}

func (f *form) prev() {
	if f.fieldCount() == 0 {
		return
	}
	f.focus--
	if f.focus < 0 {
		f.focus = f.fieldCount() - 1
	}
	f.focusCurrent()
}

// update passes the message to whichever field currently has focus.
func (f form) update(msg tea.Msg) (form, tea.Cmd) {
	var cmd tea.Cmd
	if f.focus < len(f.inputs) {
		f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	} else if f.hasBody {
		f.body, cmd = f.body.Update(msg)
	}
	return f, cmd
}

// values splits the filled-in fields back out by where they belong.
func (f form) values() (path, query, headers map[string]string) {
	path = map[string]string{}
	query = map[string]string{}
	headers = map[string]string{}

	for i, p := range f.endpoint.Params {
		if i >= len(f.inputs) {
			break
		}
		v := strings.TrimSpace(f.inputs[i].Value())
		switch p.In {
		case "path":
			path[p.Name] = v
		case "query":
			query[p.Name] = v
		case "header":
			headers[p.Name] = v
		}
	}
	return path, query, headers
}

// missingRequired reports which required fields are still blank, so we can
// stop the user before firing a request that's guaranteed to 400.
func (f form) missingRequired() []string {
	var missing []string
	for i, p := range f.endpoint.Params {
		if i >= len(f.inputs) {
			break
		}
		if p.Required && strings.TrimSpace(f.inputs[i].Value()) == "" {
			missing = append(missing, p.Name)
		}
	}
	return missing
}

func (f form) bodyValue() string {
	if !f.hasBody {
		return ""
	}
	return f.body.Value()
}

func (f form) view() string {
	var b strings.Builder

	b.WriteString(methodStyle(f.endpoint.Method).Render(" "+f.endpoint.Method+" ") + " ")
	b.WriteString(pathStyle.Render(f.endpoint.Path) + "\n")
	if f.endpoint.Summary != "" {
		b.WriteString(dimStyle.Render(truncate(f.endpoint.Summary, 44)) + "\n")
	}
	b.WriteString("\n")

	if f.fieldCount() == 0 {
		b.WriteString(dimStyle.Render("No parameters. Press ctrl+s to send.") + "\n")
		return b.String()
	}

	for i, p := range f.endpoint.Params {
		if i >= len(f.inputs) {
			break
		}
		label := fmt.Sprintf("%s (%s)", p.Name, p.In)
		if p.Required {
			label += requiredStyle.Render(" *")
		}
		if i == f.focus {
			b.WriteString(selStyle.Render("› "+label) + "\n")
		} else {
			b.WriteString("  " + dimStyle.Render(label) + "\n")
		}
		b.WriteString("  " + f.inputs[i].View() + "\n\n")
	}

	if f.hasBody {
		label := "body (json)"
		if f.focus == len(f.inputs) {
			b.WriteString(selStyle.Render("› "+label) + "\n")
		} else {
			b.WriteString("  " + dimStyle.Render(label) + "\n")
		}
		b.WriteString(f.body.View() + "\n")
	}

	return b.String()
}
