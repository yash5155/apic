package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/yash5155/apic/internal/spec"
)

// form is built at runtime from an endpoint's parameters.
//
// This is the real difference from a hand-written TUI: you don't know how
// many inputs there are until the user picks an endpoint, so the fields live
// in a slice and focus is just an index into it. Each field carries its own
// spec.Param (see field.go), so text and enum inputs — and synthetic auth
// fields — all coexist in one focus-ordered slice.
type form struct {
	endpoint spec.Endpoint
	fields   []field
	body     textarea.Model
	hasBody  bool
	focus    int // 0..len(fields)-1 = a field, len(fields) = the body
}

// newForm builds a form for an endpoint. sec and base drive security-scheme
// auto-fill: for each scheme the endpoint requires, an auth field is added
// (and prefilled from the global -H headers where possible).
func newForm(ep spec.Endpoint, sec map[string]spec.SecurityScheme, base map[string]string) form {
	f := form{
		endpoint: ep,
		hasBody:  ep.BodySkeleton != "",
	}

	for _, p := range ep.Params {
		f.fields = append(f.fields, newField(p))
	}

	f.addAuthFields(sec, base)

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

// addAuthFields synthesizes an input for each security scheme the endpoint
// requires, unless the spec already declared it as a parameter. http/bearer and
// basic auth become an Authorization header, prefilled from a global -H token
// when one is present.
func (f *form) addAuthFields(sec map[string]spec.SecurityScheme, base map[string]string) {
	for _, key := range f.endpoint.Auth {
		s, ok := sec[key]
		if !ok {
			continue
		}
		switch s.Type {
		case "apiKey":
			if (s.In != "header" && s.In != "query") || s.Name == "" || f.hasParam(s.Name, s.In) {
				continue
			}
			p := spec.Param{Name: s.Name, In: s.In, Required: true, Type: "token", Note: "(auth)"}
			f.fields = append(f.fields, newField(p))
		case "http", "oauth2", "openIdConnect":
			if f.hasParam("Authorization", "header") {
				continue
			}
			hint := "token"
			switch s.Scheme {
			case "bearer", "":
				hint = "Bearer <token>"
			case "basic":
				hint = "Basic <base64>"
			}
			p := spec.Param{Name: "Authorization", In: "header", Required: true, Type: hint, Note: "(auth)"}
			fld := newField(p)
			if v := base["Authorization"]; v != "" {
				fld.SetValue(v) // reuse the global -H token
			}
			f.fields = append(f.fields, fld)
		}
	}
}

// placeholderFor turns schema info into a useful hint.
func placeholderFor(p spec.Param) string {
	if len(p.Enum) > 0 {
		return strings.Join(p.Enum, " | ")
	}
	return p.Type
}

// hasParam reports whether a field for this name+location already exists, so
// auto-fill doesn't duplicate a parameter the spec already declared.
func (f form) hasParam(name, in string) bool {
	for _, fld := range f.fields {
		p := fld.Param()
		if p.Name == name && p.In == in {
			return true
		}
	}
	return false
}

// fieldCount includes the body pane if there is one.
func (f form) fieldCount() int {
	if f.hasBody {
		return len(f.fields) + 1
	}
	return len(f.fields)
}

// focusCurrent blurs everything, then focuses whatever focus points at.
// Calling this after every focus change is simpler than tracking which
// field used to be focused.
func (f *form) focusCurrent() {
	for i := range f.fields {
		f.fields[i].Blur()
	}
	if f.hasBody {
		f.body.Blur()
	}

	if f.focus < len(f.fields) {
		f.fields[f.focus].Focus()
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

// bodyFocused reports whether the JSON body pane currently has focus. Used to
// avoid stealing textarea editing keys (ctrl+e/f/b/…) for global shortcuts.
func (f form) bodyFocused() bool {
	return f.hasBody && f.focus == len(f.fields)
}

// update passes the message to whichever field currently has focus.
func (f form) update(msg tea.Msg) (form, tea.Cmd) {
	var cmd tea.Cmd
	if f.focus < len(f.fields) {
		f.fields[f.focus], cmd = f.fields[f.focus].Update(msg)
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

	for _, fld := range f.fields {
		p := fld.Param()
		v := fld.Value()
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

// setValue fills the field with the given parameter name, if present. Used by
// request-history reload.
func (f *form) setValue(name, v string) {
	for _, fld := range f.fields {
		if fld.Param().Name == name {
			fld.SetValue(v)
			return
		}
	}
}

// value returns the current value of the field with the given name.
func (f form) value(name string) string {
	for _, fld := range f.fields {
		if fld.Param().Name == name {
			return fld.Value()
		}
	}
	return ""
}

// missingRequired reports which required fields are still blank, so we can
// stop the user before firing a request that's guaranteed to 400.
func (f form) missingRequired() []string {
	var missing []string
	for _, fld := range f.fields {
		p := fld.Param()
		if p.Required && fld.Value() == "" {
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

	for i, fld := range f.fields {
		p := fld.Param()
		label := fmt.Sprintf("%s (%s)", p.Name, p.In)
		if p.Required {
			label += requiredStyle.Render(" *")
		}
		if p.Note != "" {
			label += dimStyle.Render(" " + p.Note)
		}
		if i == f.focus {
			b.WriteString(selStyle.Render("› "+label) + "\n")
		} else {
			b.WriteString("  " + dimStyle.Render(label) + "\n")
		}
		b.WriteString("  " + fld.View() + "\n\n")
	}

	if f.hasBody {
		label := "body (json)"
		if f.focus == len(f.fields) {
			b.WriteString(selStyle.Render("› "+label) + "\n")
		} else {
			b.WriteString("  " + dimStyle.Render(label) + "\n")
		}
		b.WriteString(f.body.View() + "\n")
	}

	return b.String()
}
