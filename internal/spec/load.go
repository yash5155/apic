// Loading and version handling live here, separate from the flattening in
// spec.go. The goal of this file is to accept anything a user is likely to
// throw at it — OpenAPI 3.0/3.1 or Swagger 2.0, JSON or YAML, a local path
// or an http(s) URL — and hand spec.go a single resolved OpenAPI 3 document.
package spec

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/invopop/yaml"
)

// Fetch configuration for remote specs. The CLI overrides these from flags
// before calling Load; the defaults are safe for interactive use.
var (
	// FetchTimeout bounds how long we wait for a remote spec to download.
	FetchTimeout = 30 * time.Second
	// MaxSpecBytes caps the size of any spec we will read, remote or local,
	// so a hostile or accidental multi-gigabyte file cannot exhaust memory.
	MaxSpecBytes int64 = 32 << 20 // 32 MiB
	// InsecureTLS disables certificate verification when fetching a remote
	// spec over https. Off by default; opt in with --insecure.
	InsecureTLS = false
)

// Load reads, converts and flattens a spec from a file path or http(s) URL.
func Load(source string) (*API, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("no spec source given")
	}

	data, err := readSource(source)
	if err != nil {
		return nil, err
	}

	doc, err := parseDoc(data, source, isRemote(source))
	if err != nil {
		return nil, err
	}

	// Validation is advisory. Plenty of real-world specs are slightly invalid
	// but still perfectly usable, so we surface endpoints regardless.
	_ = doc.Validate(context.Background())

	return flatten(doc), nil
}

// readSource returns the raw bytes of a spec, enforcing the size cap.
func readSource(source string) ([]byte, error) {
	if isRemote(source) {
		return fetchRemote(source)
	}
	return readLocal(source)
}

func isRemote(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}

// readLocal reads a spec file, refusing anything larger than the cap.
func readLocal(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open spec file: %w", err)
	}
	defer f.Close()

	if info, err := f.Stat(); err == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("%s is a directory, not a spec file", path)
		}
		if info.Size() > MaxSpecBytes {
			return nil, fmt.Errorf("spec file is %d bytes, over the %d byte limit", info.Size(), MaxSpecBytes)
		}
	}

	data, err := io.ReadAll(io.LimitReader(f, MaxSpecBytes+1))
	if err != nil {
		return nil, fmt.Errorf("could not read spec file: %w", err)
	}
	if int64(len(data)) > MaxSpecBytes {
		return nil, fmt.Errorf("spec file exceeds the %d byte limit", MaxSpecBytes)
	}
	return data, nil
}

// fetchRemote downloads a spec with an explicit timeout, size limit and TLS
// policy. We deliberately do our own fetch rather than letting the OpenAPI
// loader reach out, so all network behaviour is controlled in one place.
func fetchRemote(rawURL string) ([]byte, error) {
	u, err := parseURL(rawURL)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: FetchTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: InsecureTLS}, //nolint:gosec // gated behind explicit --insecure
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching spec returned HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxSpecBytes+1))
	if err != nil {
		return nil, fmt.Errorf("could not read spec response: %w", err)
	}
	if int64(len(data)) > MaxSpecBytes {
		return nil, fmt.Errorf("remote spec exceeds the %d byte limit", MaxSpecBytes)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("remote spec is empty; is %s a Swagger UI page rather than the raw spec?", rawURL)
	}
	return data, nil
}

// parseDoc detects the spec flavour and returns a resolved OpenAPI 3 document.
func parseDoc(data []byte, source string, remote bool) (*openapi3.T, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("spec is empty")
	}

	// Normalise to JSON purely for version sniffing. YAMLToJSON is a no-op on
	// input that is already JSON, so this handles both formats.
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("spec is not valid JSON or YAML: %w", err)
	}

	var probe struct {
		Swagger string `json:"swagger"`
		OpenAPI string `json:"openapi"`
	}
	_ = json.Unmarshal(jsonData, &probe)

	switch {
	case strings.HasPrefix(probe.Swagger, "2."):
		return convertV2(jsonData)
	case strings.HasPrefix(probe.OpenAPI, "3."):
		return loadV3(data, source, remote)
	default:
		// No recognisable version field. Try the v3 loader anyway — some tools
		// emit specs without the marker — and give a clear error if it fails.
		doc, err := loadV3(data, source, remote)
		if err != nil {
			return nil, fmt.Errorf(`unrecognised spec: no "openapi" (3.x) or "swagger" (2.0) version field found`)
		}
		return doc, nil
	}
}

// convertV2 turns a Swagger 2.0 document into OpenAPI 3. This is the path that
// makes the tool work against the huge population of 2.0 specs still in the
// wild, which the openapi3 parser cannot read directly.
func convertV2(jsonData []byte) (*openapi3.T, error) {
	var doc2 openapi2.T
	if err := json.Unmarshal(jsonData, &doc2); err != nil {
		return nil, fmt.Errorf("invalid Swagger 2.0 document: %w", err)
	}

	v3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return nil, fmt.Errorf("could not convert Swagger 2.0 to OpenAPI 3: %w", err)
	}

	// Populate the .Value on any $refs the conversion left behind so the
	// flattener never sees a bare reference.
	loader := openapi3.NewLoader()
	_ = loader.ResolveRefsIn(v3, nil)
	return v3, nil
}

// loadV3 parses an OpenAPI 3.x document. For local files we let the loader use
// the file's directory as a base so relative external $refs resolve. For
// remote specs we forbid external refs entirely: chasing a $ref to an
// arbitrary URL would turn this tool into an SSRF vector.
func loadV3(data []byte, source string, remote bool) (*openapi3.T, error) {
	loader := openapi3.NewLoader()

	if remote {
		loader.IsExternalRefsAllowed = false
		doc, err := loader.LoadFromData(data)
		if err != nil {
			return nil, fmt.Errorf("could not parse OpenAPI document: %w", err)
		}
		return doc, nil
	}

	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromFile(source)
	if err != nil {
		// Fall back to parsing the bytes we already have (e.g. the source was
		// piped or the extension confused the loader).
		if doc2, derr := loader.LoadFromData(data); derr == nil {
			return doc2, nil
		}
		return nil, fmt.Errorf("could not parse OpenAPI document: %w", err)
	}
	return doc, nil
}
