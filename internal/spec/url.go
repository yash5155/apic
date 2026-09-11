package spec

import (
	"fmt"
	"net/url"
	"strings"
)

// parseURL validates a remote spec URL. It rejects anything that is not a
// well-formed absolute http(s) URL so callers get a clear message instead of
// an opaque failure deep inside the HTTP stack.
func parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid url %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q (want http or https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url %q has no host", raw)
	}
	return u, nil
}
