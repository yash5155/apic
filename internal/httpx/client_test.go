package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildSubstitutesPathParams(t *testing.T) {
	r := Request{
		Method:     "GET",
		BaseURL:    "https://api.test/v1/",
		Path:       "/pets/{petId}",
		PathParams: map[string]string{"petId": "42"},
		Query:      map[string]string{"limit": "10", "status": ""},
	}

	req, err := r.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// note the trailing slash on BaseURL must not produce a double slash
	if got := req.URL.Path; got != "/v1/pets/42" {
		t.Errorf("path = %q", got)
	}
	// empty query values are skipped, not sent as status=
	if got := req.URL.RawQuery; got != "limit=10" {
		t.Errorf("query = %q", got)
	}
}

func TestBuildRejectsMissingPathParam(t *testing.T) {
	r := Request{
		Method:     "GET",
		BaseURL:    "https://api.test",
		Path:       "/pets/{petId}",
		PathParams: map[string]string{"petId": ""},
	}
	if _, err := r.Build(); err == nil {
		t.Fatal("expected an error for a blank path param")
	}
}

func TestBuildEscapesPathParams(t *testing.T) {
	r := Request{
		Method:     "GET",
		BaseURL:    "https://api.test",
		Path:       "/pets/{name}",
		PathParams: map[string]string{"name": "good boy"},
	}
	req, err := r.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(req.URL.String(), "good%20boy") {
		t.Errorf("space not escaped: %s", req.URL.String())
	}
}

func TestSendAgainstRealServer(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody, gotHeader, gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotHeader = r.Header.Get("X-Request-Id")
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// deliberately unindented, to prove we pretty-print it
		w.Write([]byte(`{"id":7,"name":"Rex"}`))
	}))
	defer srv.Close()

	res := Send(Request{
		Method:  "POST",
		BaseURL: srv.URL,
		Path:    "/pets",
		Query:   map[string]string{"dry": "true"},
		Headers: map[string]string{"X-Request-Id": "abc-123"},
		Body:    `{"name":"Rex"}`,
	})

	if res.Err != nil {
		t.Fatalf("Send: %v", res.Err)
	}
	if res.Status != http.StatusCreated {
		t.Errorf("status = %d", res.Status)
	}
	if gotMethod != "POST" || gotPath != "/pets" || gotQuery != "dry=true" {
		t.Errorf("server saw %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if gotHeader != "abc-123" {
		t.Errorf("custom header = %q", gotHeader)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q", gotContentType)
	}
	if gotBody != `{"name":"Rex"}` {
		t.Errorf("body = %q", gotBody)
	}
	if res.Duration <= 0 {
		t.Error("duration should be measured")
	}

	// the response must come back indented
	if !strings.Contains(res.Body, "\n  \"id\": 7") {
		t.Errorf("response was not pretty-printed:\n%s", res.Body)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.Body), &parsed); err != nil {
		t.Errorf("pretty-printing broke the json: %v", err)
	}
}

func TestSendReportsConnectionError(t *testing.T) {
	// port 1 is reserved and nothing listens there
	res := Send(Request{Method: "GET", BaseURL: "http://127.0.0.1:1", Path: "/"})
	if res.Err == nil {
		t.Fatal("expected a connection error")
	}
}

func TestNonJSONBodyPassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain text, not json"))
	}))
	defer srv.Close()

	res := Send(Request{Method: "GET", BaseURL: srv.URL, Path: "/"})
	if res.Body != "plain text, not json" {
		t.Errorf("body = %q", res.Body)
	}
}
