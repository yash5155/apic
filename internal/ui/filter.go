package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// applyDotPath evaluates a lightweight jq-like path against a JSON body and
// returns the selected node as pretty JSON. Supported syntax:
//
//	.              whole document
//	.key           object member
//	.a.b.c         nested members
//	.items[0]      array index
//	.a.b[2].name   mixed
//
// It intentionally avoids a full jq dependency; it covers navigation, which is
// what the response pane needs. Errors are returned as readable messages.
func applyDotPath(body, path string) (string, error) {
	cur, err := resolvePath(body, path)
	if err != nil {
		return "", err
	}
	out, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// resolvePath parses body as JSON and walks it to the node named by a dot-path,
// returning the decoded value. Shared by applyDotPath (which pretty-prints it)
// and extractValue (which formats it as a raw scalar for chaining).
func resolvePath(body, path string) (any, error) {
	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil, fmt.Errorf("response is not JSON")
	}

	segs, err := parsePath(path)
	if err != nil {
		return nil, err
	}

	cur := root
	for _, s := range segs {
		switch s.kind {
		case segKey:
			obj, ok := cur.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("cannot read .%s: not an object", s.key)
			}
			v, ok := obj[s.key]
			if !ok {
				return nil, fmt.Errorf("no such key %q", s.key)
			}
			cur = v
		case segIndex:
			arr, ok := cur.([]any)
			if !ok {
				return nil, fmt.Errorf("cannot index [%d]: not an array", s.idx)
			}
			if s.idx < 0 || s.idx >= len(arr) {
				return nil, fmt.Errorf("index [%d] out of range (len %d)", s.idx, len(arr))
			}
			cur = arr[s.idx]
		}
	}
	return cur, nil
}

// extractValue resolves a dot-path and formats the leaf as a plain string
// suitable for reuse in a URL, header or body (request chaining). Scalars come
// back without JSON quoting; objects/arrays fall back to compact JSON.
func extractValue(body, path string) (string, error) {
	v, err := resolvePath(body, path)
	if err != nil {
		return "", err
	}
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	case bool:
		return strconv.FormatBool(t), nil
	case float64:
		// Trim the trailing ".0" JSON gives every integral number.
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case json.Number:
		return t.String(), nil
	default:
		out, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
}

type segKind int

const (
	segKey segKind = iota
	segIndex
)

type segment struct {
	kind segKind
	key  string
	idx  int
}

// parsePath turns ".a.b[0].c" into an ordered list of segments. An empty path
// or "." selects the whole document (no segments).
func parsePath(path string) ([]segment, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return nil, nil
	}
	if !strings.HasPrefix(path, ".") && !strings.HasPrefix(path, "[") {
		return nil, fmt.Errorf("path must start with '.' (e.g. .data.items[0])")
	}

	var segs []segment
	i := 0
	for i < len(path) {
		switch path[i] {
		case '.':
			i++
			start := i
			for i < len(path) && path[i] != '.' && path[i] != '[' {
				i++
			}
			key := path[start:i]
			if key == "" {
				return nil, fmt.Errorf("empty key in path")
			}
			segs = append(segs, segment{kind: segKey, key: key})
		case '[':
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("missing ] in path")
			}
			numStr := path[i+1 : i+end]
			n, err := strconv.Atoi(strings.TrimSpace(numStr))
			if err != nil {
				return nil, fmt.Errorf("invalid array index %q", numStr)
			}
			segs = append(segs, segment{kind: segIndex, idx: n})
			i += end + 1
		default:
			return nil, fmt.Errorf("unexpected character %q in path", string(path[i]))
		}
	}
	return segs, nil
}
