package ui

import (
	"fmt"
	"strings"
)

// effectiveVars is the variable set used for interpolation: the environment's
// variables (built-ins < file < --var) overlaid with values captured from
// responses this session. Captured values win, so the most recent capture is
// what later requests use.
func (m Model) effectiveVars() map[string]string {
	base := m.envs.Vars()
	if len(m.captured) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(m.captured))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range m.captured {
		out[k] = v
	}
	return out
}

// doCapture parses a "name = .dot.path" line, extracts the value from the last
// response, stores it as a captured variable, and returns a status message.
func (m *Model) doCapture(input string) string {
	if m.result == nil || m.result.Err != nil {
		return "no response to capture from"
	}

	name, path, err := parseCapture(input)
	if err != nil {
		return err.Error()
	}

	value, err := extractValue(m.result.Body, path)
	if err != nil {
		return "capture failed: " + err.Error()
	}

	if m.captured == nil {
		m.captured = map[string]string{}
	}
	m.captured[name] = value

	return fmt.Sprintf("captured {{%s}} = %s", name, truncate(value, 40))
}

// parseCapture splits "name = path" (or "name path"). The path defaults to "."
// (the whole document) when only a name is given.
func parseCapture(input string) (name, path string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("usage: name = .path")
	}

	if n, p, ok := strings.Cut(input, "="); ok {
		name, path = strings.TrimSpace(n), strings.TrimSpace(p)
	} else if n, p, ok := strings.Cut(input, " "); ok {
		name, path = strings.TrimSpace(n), strings.TrimSpace(p)
	} else {
		name = input
	}

	if name == "" {
		return "", "", fmt.Errorf("capture needs a variable name")
	}
	if path == "" {
		path = "."
	}
	return name, path, nil
}
