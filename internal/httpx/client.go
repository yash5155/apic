// Package httpx builds and sends the actual request.
//
// Nothing in here knows about Bubble Tea. That matters: Send is a plain
// blocking function, which makes it trivial to test, and the UI layer
// wraps it in a tea.Cmd to get it off the main loop.
package httpx

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Request is everything we need to fire one call.
type Request struct {
	Method     string
	BaseURL    string
	Path       string // still contains {placeholders}
	PathParams map[string]string
	Query      map[string]string
	Headers    map[string]string
	Body       string
}

// Result is what comes back. Err is set instead of the rest on failure.
type Result struct {
	Status    int
	Duration  time.Duration
	Body      string
	Headers   http.Header
	Truncated bool // true if the body was cut off at the size cap
	Err       error
}

// Config controls transport behaviour. The CLI sets it once at startup from
// flags; callers that don't care get sane defaults via the package globals.
type Config struct {
	Timeout      time.Duration
	MaxBodyBytes int64
	InsecureTLS  bool
}

// Sensible defaults for interactive use.
const (
	defaultTimeout = 30 * time.Second
	defaultMaxBody = 2 << 20 // 2 MiB
)

var (
	mu      sync.RWMutex
	cfg     = Config{Timeout: defaultTimeout, MaxBodyBytes: defaultMaxBody}
	client  = buildClient(cfg)
	maxBody = int64(defaultMaxBody)
)

// Configure applies transport settings. Zero-valued fields keep their default.
func Configure(c Config) {
	mu.Lock()
	defer mu.Unlock()
	if c.Timeout > 0 {
		cfg.Timeout = c.Timeout
	}
	if c.MaxBodyBytes > 0 {
		cfg.MaxBodyBytes = c.MaxBodyBytes
	}
	cfg.InsecureTLS = c.InsecureTLS
	client = buildClient(cfg)
	maxBody = cfg.MaxBodyBytes
}

// buildClient constructs an http.Client with our redirect and TLS policy.
func buildClient(c Config) *http.Client {
	return &http.Client{
		Timeout: c.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.InsecureTLS}, //nolint:gosec // gated behind explicit --insecure
			Proxy:           http.ProxyFromEnvironment,
		},
		// Cap redirects and strip credentials when the host changes, so an
		// endpoint cannot bounce our Authorization header to a third party.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if len(via) > 0 && req.URL.Host != via[0].URL.Host {
				req.Header.Del("Authorization")
				req.Header.Del("Cookie")
			}
			return nil
		},
	}
}

// Build assembles the http.Request, substituting path params and
// appending query params.
func (r Request) Build() (*http.Request, error) {
	if strings.TrimSpace(r.Method) == "" {
		return nil, fmt.Errorf("request method is empty")
	}
	if strings.TrimSpace(r.BaseURL) == "" {
		return nil, fmt.Errorf("no base URL; pass --server https://api.example.com")
	}

	path := r.Path
	for name, value := range r.PathParams {
		if value == "" {
			return nil, fmt.Errorf("path parameter %q is required", name)
		}
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(value))
	}

	full := strings.TrimRight(r.BaseURL, "/") + path
	u, err := url.Parse(full)
	if err != nil {
		return nil, fmt.Errorf("bad url %q: %w", full, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q in %q (want http or https)", u.Scheme, full)
	}

	q := u.Query()
	for name, value := range r.Query {
		if value != "" { // empty means "user skipped it"
			q.Set(name, value)
		}
	}
	u.RawQuery = q.Encode()

	var body io.Reader
	if strings.TrimSpace(r.Body) != "" {
		body = strings.NewReader(r.Body)
	}

	req, err := http.NewRequest(strings.ToUpper(r.Method), u.String(), body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for name, value := range r.Headers {
		if value != "" {
			req.Header.Set(name, value)
		}
	}

	return req, nil
}

// Send performs the request and always returns a Result, never a bare
// error, so the caller has one thing to render either way.
func Send(r Request) Result {
	req, err := r.Build()
	if err != nil {
		return Result{Err: err}
	}

	mu.RLock()
	c, cap := client, maxBody
	mu.RUnlock()

	start := time.Now()
	resp, err := c.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return Result{Err: err, Duration: elapsed}
	}
	defer resp.Body.Close()

	// Read one byte past the cap so we can tell whether we truncated.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, cap+1))
	if err != nil {
		return Result{Err: err, Duration: elapsed, Status: resp.StatusCode}
	}
	truncated := int64(len(raw)) > cap
	if truncated {
		raw = raw[:cap]
	}

	return Result{
		Status:    resp.StatusCode,
		Duration:  elapsed,
		Body:      prettyJSON(raw),
		Headers:   resp.Header,
		Truncated: truncated,
	}
}

// prettyJSON indents the body if it is JSON, otherwise returns it as-is.
func prettyJSON(raw []byte) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "(empty body)"
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}

// StatusText gives us "200 OK" style labels for the header line.
func StatusText(code int) string {
	if code == 0 {
		return ""
	}
	return fmt.Sprintf("%d %s", code, http.StatusText(code))
}
