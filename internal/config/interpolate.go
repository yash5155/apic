package config

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// varPattern matches {{name}} — DOUBLE braces only. The restricted character
// class means it can never match OpenAPI's single-brace {pathParam} syntax, so
// the two substitution mechanisms never interfere. Whitespace inside the braces
// is tolerated: {{ name }}.
var varPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}`)

// Interpolate replaces {{name}} placeholders in s using vars. A name prefixed
// with "env." is resolved from the process environment (e.g. {{env.TOKEN}}), so
// secrets can stay out of the config file. Unknown placeholders are left in the
// output verbatim AND reported in unresolved (de-duplicated, sorted), so the
// caller can refuse to send a half-filled request.
func Interpolate(s string, vars map[string]string) (out string, unresolved []string) {
	miss := map[string]bool{}

	out = varPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := strings.TrimSpace(varPattern.FindStringSubmatch(match)[1])

		if strings.HasPrefix(name, "env.") {
			if v, ok := os.LookupEnv(strings.TrimPrefix(name, "env.")); ok {
				return v
			}
			miss[name] = true
			return match
		}
		if v, ok := vars[name]; ok {
			return v
		}
		miss[name] = true
		return match
	})

	if len(miss) > 0 {
		unresolved = make([]string, 0, len(miss))
		for name := range miss {
			unresolved = append(unresolved, name)
		}
		sort.Strings(unresolved)
	}
	return out, unresolved
}

// InterpolateMap applies Interpolate to every value in m, returning a new map
// and the aggregated unresolved names.
func InterpolateMap(m, vars map[string]string) (map[string]string, []string) {
	if len(m) == 0 {
		return m, nil
	}
	out := make(map[string]string, len(m))
	seen := map[string]bool{}
	var unresolved []string
	for k, v := range m {
		ev, miss := Interpolate(v, vars)
		out[k] = ev
		for _, name := range miss {
			if !seen[name] {
				seen[name] = true
				unresolved = append(unresolved, name)
			}
		}
	}
	sort.Strings(unresolved)
	return out, unresolved
}

// MergeVars builds the effective variable set: built-ins < file vars < --var
// overrides. It returns a fresh map and never mutates its inputs.
func MergeVars(fileVars, cliVars map[string]string) map[string]string {
	out := builtins()
	for k, v := range fileVars {
		out[k] = v
	}
	for k, v := range cliVars {
		out[k] = v
	}
	return out
}

// builtins are convenience variables always available (lowest precedence).
func builtins() map[string]string {
	return map[string]string{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
}
