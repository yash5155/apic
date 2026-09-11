package ui

import (
	"sort"

	"github.com/yash5155/apic/internal/config"
	"github.com/yash5155/apic/internal/httpx"
)

// EnvSet holds the active-environment state for a session: the loaded config,
// the ordered environment names, which one is active, and the effective
// variables/headers derived from it. It is nil-safe — a nil *EnvSet means no
// environments are in play, so all env features are simply inert.
type EnvSet struct {
	cfg          *config.Config
	names        []string          // sorted; the switch order
	active       int               // index into names, or -1 for none
	cliVars      map[string]string // --var overrides, re-applied on switch
	vars         map[string]string // effective vars (built-ins < file < --var)
	headers      map[string]string // active env's default headers
	serverForced bool              // --server given: don't change base on switch
}

// NewEnvSet builds the env state from a loaded config and the chosen active
// environment. active may be "" (no environment); cliVars are the --var flags.
func NewEnvSet(cfg *config.Config, active string, cliVars map[string]string, serverForced bool) *EnvSet {
	names := cfg.Names()
	idx := -1
	for i, n := range names {
		if n == active {
			idx = i
		}
	}
	e := &EnvSet{cfg: cfg, names: names, active: idx, cliVars: cliVars, serverForced: serverForced}
	e.load()
	return e
}

// load recomputes vars and headers for the current active index.
func (e *EnvSet) load() {
	var env config.Environment
	if e.active >= 0 {
		env, _ = e.cfg.Env(e.names[e.active])
	}
	e.vars = config.MergeVars(env.Vars, e.cliVars)
	e.headers = env.Headers
}

// ActiveName returns the active environment name, or "" if none.
func (e *EnvSet) ActiveName() string {
	if e == nil || e.active < 0 {
		return ""
	}
	return e.names[e.active]
}

// HasMultiple reports whether there is more than one environment to switch between.
func (e *EnvSet) HasMultiple() bool { return e != nil && len(e.names) > 1 }

// BaseURL returns the active environment's base URL, or "".
func (e *EnvSet) BaseURL() string {
	if e == nil || e.active < 0 {
		return ""
	}
	env, _ := e.cfg.Env(e.names[e.active])
	return env.BaseURL
}

// Vars returns the effective session variables.
func (e *EnvSet) Vars() map[string]string {
	if e == nil {
		return nil
	}
	return e.vars
}

// Headers returns the active environment's default headers.
func (e *EnvSet) Headers() map[string]string {
	if e == nil {
		return nil
	}
	return e.headers
}

// next advances to the following environment and reloads vars/headers.
func (e *EnvSet) next() {
	if e == nil || len(e.names) == 0 {
		return
	}
	e.active = (e.active + 1) % len(e.names)
	e.load()
}

// cycleEnv switches to the next environment, updating the model's vars, env
// headers and (unless --server forced the base) the base URL.
func (m *Model) cycleEnv() {
	if !m.envs.HasMultiple() {
		return
	}
	m.envs.next()
	m.vars = m.envs.Vars()
	m.envHeaders = m.envs.Headers()
	if !m.envs.serverForced {
		if b := m.envs.BaseURL(); b != "" {
			if expanded, missing := config.Interpolate(b, m.effectiveVars()); len(missing) == 0 {
				m.baseURL = expanded
			}
		}
	}
	m.errMsg = "env: " + m.envs.ActiveName()
}

// buildRequest assembles the request from the current form and interpolates all
// {{var}} placeholders using the active environment's variables. The returned
// slice lists any placeholders that had no value; when non-empty the caller
// must not send (mirrors body-validation blocking).
//
// Header layering is env headers < global -H (baseHeaders) < form/auth fields.
// The path template stays raw — only the values the user typed are interpolated,
// so apic's {{var}} never collides with OpenAPI's {pathParam} (resolved later
// in httpx.Build).
func (m Model) buildRequest() (httpx.Request, []string) {
	path, query, headers := m.form.values()
	merged := mergeHeaders(mergeHeaders(m.envHeaders, m.baseHeaders), headers)

	vars := m.effectiveVars()
	var unresolved []string
	add := func(names []string) {
		unresolved = append(unresolved, names...)
	}

	base, miss := config.Interpolate(m.baseURL, vars)
	add(miss)
	pathParams, miss := config.InterpolateMap(path, vars)
	add(miss)
	q, miss := config.InterpolateMap(query, vars)
	add(miss)
	h, miss := config.InterpolateMap(merged, vars)
	add(miss)
	body, miss := config.Interpolate(m.form.bodyValue(), vars)
	add(miss)

	req := httpx.Request{
		Method:     m.form.endpoint.Method,
		BaseURL:    base,
		Path:       m.form.endpoint.Path,
		PathParams: pathParams,
		Query:      q,
		Headers:    h,
		Body:       body,
	}
	return req, dedupSort(unresolved)
}

// dedupSort returns the unique names in sorted order (nil if empty).
func dedupSort(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
