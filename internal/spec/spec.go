// Package spec loads an OpenAPI document and flattens it into simple
// structs of our own.
//
// This translation is the most important design decision in the project.
// kin-openapi's types are deeply nested and full of *Ref indirection. If
// those types leak into the UI, every View function ends up doing nil
// checks six levels deep. So we pay that cost exactly once, here, and the
// rest of the program only ever sees Endpoint and Param.
package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Param is one input the user has to fill in.
type Param struct {
	Name        string
	In          string // "path", "query" or "header"
	Required    bool
	Type        string // "string", "integer", "boolean", ...
	Enum        []string
	Default     string
	Description string
	Note        string // extra UI hint, e.g. "(auth)" for synthesized auth fields
}

// SecurityScheme is one declared authentication method, flattened from the
// spec's components. Only the fields the UI needs to build an input are kept.
type SecurityScheme struct {
	Key    string // component key, e.g. "ApiKeyAuth"
	Type   string // "apiKey" | "http" | "oauth2" | "openIdConnect"
	In     string // "header" | "query" (apiKey only)
	Name   string // header/query name (apiKey), or "Authorization" for http
	Scheme string // "bearer" | "basic" (http)
}

// Endpoint is one method+path pair from the spec.
type Endpoint struct {
	Method       string
	Path         string
	Summary      string
	Params       []Param
	BodySkeleton string   // pretty JSON starter body, empty if the endpoint takes none
	Auth         []string // security-scheme keys that satisfy this endpoint

	// ValidateBody checks a JSON body string against the request-body schema.
	// It is nil when the endpoint takes no JSON body, and returns "" when the
	// body is valid or a human-readable message naming the offending field.
	// Kept as a closure so kin-openapi never leaks out of this package.
	ValidateBody func(string) string `json:"-"`
}

// Label is what we show in the endpoint list.
func (e Endpoint) Label() string {
	return fmt.Sprintf("%-6s %s", e.Method, e.Path)
}

// API is the whole document, flattened.
type API struct {
	Title     string
	Version   string
	Servers   []string
	Endpoints []Endpoint
	Security  map[string]SecurityScheme // keyed by component name
}

// methodOrder keeps the list stable and readable instead of Go's random
// map iteration order.
var methodOrder = map[string]int{
	"GET": 0, "POST": 1, "PUT": 2, "PATCH": 3, "DELETE": 4,
	"HEAD": 5, "OPTIONS": 6, "TRACE": 7,
}

// flatten turns a resolved OpenAPI 3 document into our own simple structs.
// Loading and version conversion happen in load.go; by the time a document
// reaches here it is always OpenAPI 3.x with its $refs resolved.
func flatten(doc *openapi3.T) *API {
	api := &API{}
	if doc == nil {
		return api
	}
	if doc.Info != nil {
		api.Title = doc.Info.Title
		api.Version = doc.Info.Version
	}
	for _, s := range doc.Servers {
		if s != nil && s.URL != "" {
			api.Servers = append(api.Servers, s.URL)
		}
	}

	api.Security = convertSecuritySchemes(doc)

	if doc.Paths == nil {
		return api
	}

	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			ep := Endpoint{
				Method:  method,
				Path:    path,
				Summary: op.Summary,
			}
			if ep.Summary == "" {
				ep.Summary = op.Description
			}

			// Parameters declared on the path apply to every operation
			// under it, so merge both lists.
			ep.Params = append(ep.Params, convertParams(item.Parameters)...)
			ep.Params = append(ep.Params, convertParams(op.Parameters)...)
			ep.BodySkeleton = bodySkeleton(op)
			ep.ValidateBody = bodyValidator(op)
			ep.Auth = operationAuth(op, doc)

			api.Endpoints = append(api.Endpoints, ep)
		}
	}

	sort.Slice(api.Endpoints, func(i, j int) bool {
		a, b := api.Endpoints[i], api.Endpoints[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return methodOrder[a.Method] < methodOrder[b.Method]
	})

	return api
}

// convertSecuritySchemes flattens the document's declared auth schemes.
func convertSecuritySchemes(doc *openapi3.T) map[string]SecurityScheme {
	if doc.Components == nil || len(doc.Components.SecuritySchemes) == 0 {
		return nil
	}
	out := make(map[string]SecurityScheme, len(doc.Components.SecuritySchemes))
	for key, ref := range doc.Components.SecuritySchemes {
		if ref == nil || ref.Value == nil {
			continue
		}
		v := ref.Value
		s := SecurityScheme{
			Key:    key,
			Type:   v.Type,
			In:     v.In,
			Name:   v.Name,
			Scheme: strings.ToLower(v.Scheme),
		}
		// http auth carries the credential in the Authorization header.
		if s.Type == "http" || s.Type == "oauth2" || s.Type == "openIdConnect" {
			s.In = "header"
			s.Name = "Authorization"
		}
		out[key] = s
	}
	return out
}

// operationAuth returns the security-scheme keys that satisfy an operation.
// OpenAPI security is an OR of AND-groups; we take the first group as the
// pragmatic default the UI pre-fills.
func operationAuth(op *openapi3.Operation, doc *openapi3.T) []string {
	reqs := doc.Security
	if op.Security != nil {
		reqs = *op.Security
	}
	if len(reqs) == 0 {
		return nil
	}
	var keys []string
	for name := range reqs[0] {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

// bodyValidator returns a closure that validates a JSON body string against the
// operation's request-body schema. It returns nil when there is no JSON body,
// keeping all kin-openapi types contained in this package.
func bodyValidator(op *openapi3.Operation) func(string) string {
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}
	media := op.RequestBody.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil || media.Schema.Value == nil {
		return nil
	}
	schema := media.Schema.Value

	return func(body string) string {
		if strings.TrimSpace(body) == "" {
			return "" // empty body: let the server decide
		}
		var v any
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			return "invalid JSON: " + err.Error()
		}
		if err := schema.VisitJSON(v); err != nil {
			var se *openapi3.SchemaError
			if errors.As(err, &se) {
				ptr := strings.Join(se.JSONPointer(), "/")
				if ptr == "" {
					ptr = "(root)"
				}
				return fmt.Sprintf("/%s: %s", ptr, se.Reason)
			}
			return err.Error()
		}
		return ""
	}
}

func convertParams(refs openapi3.Parameters) []Param {
	var out []Param
	for _, ref := range refs {
		if ref == nil || ref.Value == nil {
			continue
		}
		p := ref.Value

		// We can't prompt for cookies sensibly, so skip them.
		if p.In == "cookie" {
			continue
		}

		out = append(out, Param{
			Name:        p.Name,
			In:          p.In,
			Required:    p.Required,
			Type:        schemaType(p.Schema),
			Enum:        schemaEnum(p.Schema),
			Default:     schemaDefault(p.Schema),
			Description: p.Description,
		})
	}
	return out
}

func schemaType(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil || ref.Value.Type == nil {
		return "string"
	}
	if t := ref.Value.Type.Slice(); len(t) > 0 {
		return t[0]
	}
	return "string"
}

func schemaEnum(ref *openapi3.SchemaRef) []string {
	if ref == nil || ref.Value == nil {
		return nil
	}
	var out []string
	for _, v := range ref.Value.Enum {
		out = append(out, fmt.Sprint(v))
	}
	return out
}

func schemaDefault(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil || ref.Value.Default == nil {
		return ""
	}
	return fmt.Sprint(ref.Value.Default)
}

// bodySkeleton turns a JSON request body schema into starter JSON so the
// user edits a real shape instead of typing braces from scratch.
func bodySkeleton(op *openapi3.Operation) string {
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return ""
	}
	media := op.RequestBody.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil || media.Schema.Value == nil {
		return ""
	}

	value := skeleton(media.Schema.Value, 0)
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(out)
}

// skeleton walks a schema and produces a placeholder value. The depth
// limit is what stops a self-referencing schema from recursing forever.
func skeleton(s *openapi3.Schema, depth int) any {
	if s == nil || depth > 6 {
		return nil
	}
	if s.Example != nil {
		return s.Example
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	if s.Type == nil {
		return ""
	}

	switch {
	case s.Type.Is("object"):
		obj := map[string]any{}
		for name, prop := range s.Properties {
			if prop != nil && prop.Value != nil {
				obj[name] = skeleton(prop.Value, depth+1)
			}
		}
		return obj
	case s.Type.Is("array"):
		if s.Items != nil && s.Items.Value != nil {
			return []any{skeleton(s.Items.Value, depth+1)}
		}
		return []any{}
	case s.Type.Is("integer"):
		return 0
	case s.Type.Is("number"):
		return 0.0
	case s.Type.Is("boolean"):
		return false
	default:
		return ""
	}
}
