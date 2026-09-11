package ui

import (
	"net/http"
	"sort"
	"strings"

	"github.com/yash5155/apic/internal/httpx"
)

// buildCurl renders an httpx.Request as an equivalent curl command. It reuses
// Request.Build so the URL, query and headers exactly match what apic sends
// (including defaulted Content-Type/Accept and any merged auth headers).
func buildCurl(r httpx.Request) (string, error) {
	req, err := r.Build()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("curl -X " + req.Method + " '" + req.URL.String() + "'")

	for _, name := range sortedHeaderNames(req.Header) {
		b.WriteString(" \\\n  -H '" + name + ": " + shellEscape(req.Header.Get(name)) + "'")
	}

	if strings.TrimSpace(r.Body) != "" {
		b.WriteString(" \\\n  -d '" + shellEscape(r.Body) + "'")
	}
	return b.String(), nil
}

// sortedHeaderNames returns header names in stable order for reproducible output.
func sortedHeaderNames(h http.Header) []string {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// shellEscape makes a value safe inside single quotes by closing the quote,
// inserting an escaped quote, and reopening: ' becomes '\”.
func shellEscape(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}
