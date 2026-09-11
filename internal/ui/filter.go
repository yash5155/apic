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
	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return "", fmt.Errorf("response is not JSON")
	}

	segs, err := parsePath(path)
	if err != nil {
		return "", err
	}

	cur := root
	for _, s := range segs {
		switch s.kind {
		case segKey:
			obj, ok := cur.(map[string]any)
			if !ok {
				return "", fmt.Errorf("cannot read .%s: not an object", s.key)
			}
			v, ok := obj[s.key]
			if !ok {
				return "", fmt.Errorf("no such key %q", s.key)
			}
			cur = v
		case segIndex:
			arr, ok := cur.([]any)
			if !ok {
				return "", fmt.Errorf("cannot index [%d]: not an array", s.idx)
			}
			if s.idx < 0 || s.idx >= len(arr) {
				return "", fmt.Errorf("index [%d] out of range (len %d)", s.idx, len(arr))
			}
			cur = arr[s.idx]
		}
	}

	out, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
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
