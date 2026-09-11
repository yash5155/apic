package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/yash5155/apic/internal/spec"
)

// field is one editable input on a form. A free-text parameter uses a text
// input; a parameter with an enum uses a selector the user cycles with the
// arrow keys and cannot type an invalid value into.
//
// Every field carries its own spec.Param, so the form no longer has to keep a
// parallel slice of params — this is what lets features like security-scheme
// auto-fill inject synthetic fields that aren't in the endpoint's declared
// parameter list.
type field interface {
	Focus() tea.Cmd
	Blur()
	Update(tea.Msg) (field, tea.Cmd)
	View() string
	Value() string
	SetValue(string)
	Param() spec.Param
}

// newField picks the right kind of field for a parameter.
func newField(p spec.Param) field {
	if len(p.Enum) > 0 {
		return newEnumField(p)
	}
	return newTextField(p)
}

// ---------------------------------------------------------------- text field

type textField struct {
	in textinput.Model
	p  spec.Param
}

func newTextField(p spec.Param) field {
	in := textinput.New()
	in.Placeholder = placeholderFor(p)
	in.Width = 34
	in.CharLimit = 1024
	if p.Default != "" {
		in.SetValue(p.Default)
	}
	return &textField{in: in, p: p}
}

func (t *textField) Focus() tea.Cmd { return t.in.Focus() }
func (t *textField) Blur()          { t.in.Blur() }

func (t *textField) Update(msg tea.Msg) (field, tea.Cmd) {
	var cmd tea.Cmd
	t.in, cmd = t.in.Update(msg)
	return t, cmd
}

func (t *textField) View() string      { return t.in.View() }
func (t *textField) Value() string     { return strings.TrimSpace(t.in.Value()) }
func (t *textField) SetValue(v string) { t.in.SetValue(v) }
func (t *textField) Param() spec.Param { return t.p }

// ---------------------------------------------------------------- enum field

// enumField constrains input to one of a fixed set of values. Index 0 is always
// the empty "unchosen" state so a required enum starts blank (and is caught by
// missingRequired) and an optional enum can be deliberately skipped.
type enumField struct {
	p       spec.Param
	options []string
	idx     int
	focused bool
}

func newEnumField(p spec.Param) field {
	opts := append([]string{""}, p.Enum...)
	e := &enumField{p: p, options: opts}
	if p.Default != "" {
		for i, o := range opts {
			if o == p.Default {
				e.idx = i
			}
		}
	}
	return e
}

func (e *enumField) Focus() tea.Cmd { e.focused = true; return nil }
func (e *enumField) Blur()          { e.focused = false }

// Update cycles the selection on the arrow keys and swallows everything else,
// so there is no way to type a value the API doesn't accept.
func (e *enumField) Update(msg tea.Msg) (field, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok || !e.focused || len(e.options) == 0 {
		return e, nil
	}
	switch key.String() {
	case "left", "up":
		e.idx = (e.idx - 1 + len(e.options)) % len(e.options)
	case "right", "down":
		e.idx = (e.idx + 1) % len(e.options)
	}
	return e, nil
}

func (e *enumField) View() string {
	val := e.options[e.idx]
	if val == "" {
		if e.p.Required {
			val = "(choose)"
		} else {
			val = "(skip)"
		}
	}
	if e.focused {
		return "‹ " + selStyle.Render(val) + " ›  " + dimStyle.Render("←/→ change")
	}
	return "  " + dimStyle.Render(val)
}

func (e *enumField) Value() string { return e.options[e.idx] }

func (e *enumField) SetValue(v string) {
	for i, o := range e.options {
		if o == v {
			e.idx = i
			return
		}
	}
}

func (e *enumField) Param() spec.Param { return e.p }
