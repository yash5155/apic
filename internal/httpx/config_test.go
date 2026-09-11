package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBodyTruncation(t *testing.T) {
	big := strings.Repeat("x", 5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(big))
	}))
	defer srv.Close()

	// Shrink the cap for the test, then restore the default afterwards.
	Configure(Config{MaxBodyBytes: 1000})
	defer Configure(Config{MaxBodyBytes: defaultMaxBody})

	res := Send(Request{Method: "GET", BaseURL: srv.URL, Path: "/"})
	if res.Err != nil {
		t.Fatalf("Send: %v", res.Err)
	}
	if !res.Truncated {
		t.Error("expected Truncated to be set")
	}
	if len(res.Body) != 1000 {
		t.Errorf("body length = %d, want 1000", len(res.Body))
	}
}

func TestConfigureKeepsDefaultsOnZero(t *testing.T) {
	Configure(Config{}) // all zero
	mu.RLock()
	defer mu.RUnlock()
	if cfg.Timeout != defaultTimeout {
		t.Errorf("timeout = %v, want default", cfg.Timeout)
	}
	if cfg.MaxBodyBytes != defaultMaxBody {
		t.Errorf("maxBody = %d, want default", cfg.MaxBodyBytes)
	}
}

func TestConfigureAppliesTimeout(t *testing.T) {
	Configure(Config{Timeout: 5 * time.Second})
	defer Configure(Config{Timeout: defaultTimeout})
	mu.RLock()
	defer mu.RUnlock()
	if client.Timeout != 5*time.Second {
		t.Errorf("client timeout = %v", client.Timeout)
	}
}

func TestBuildRejectsBadScheme(t *testing.T) {
	_, err := Request{Method: "GET", BaseURL: "ftp://x.test", Path: "/a"}.Build()
	if err == nil {
		t.Error("expected an error for a non-http scheme")
	}
}

func TestBuildRejectsEmptyBaseURL(t *testing.T) {
	_, err := Request{Method: "GET", Path: "/a"}.Build()
	if err == nil {
		t.Error("expected an error for an empty base URL")
	}
}
