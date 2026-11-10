// Package strutil holds allocation-free string scans shared by Krit's
// rules and CLI commands. Hand-rolled equivalents of small regex
// patterns belong here when the regex shows up in a per-line or
// per-token hot path.
package strutil

import "strings"

// ContainsTokenWordBoundary reports whether token appears in text
// bounded by non-identifier characters (or string start/end).
// Identifier characters are [A-Za-z0-9_]. Equivalent to the regex
// `(^|[^A-Za-z0-9_])\Q<token>\E([^A-Za-z0-9_]|$)` but with no
// per-call regex compile and no allocation.
//
// An empty token returns false; this matches the regex form
// (`regexp.QuoteMeta("")` is the empty string and `(^|x)([^x]|$)`
// would degenerate to a position-only match the callers do not want).
func ContainsTokenWordBoundary(text, token string) bool {
	if token == "" {
		return false
	}
	from := 0
	for from <= len(text) {
		rel := strings.Index(text[from:], token)
		if rel < 0 {
			return false
		}
		pos := from + rel
		end := pos + len(token)
		leftOK := pos == 0 || !isIdentByte(text[pos-1])
		rightOK := end == len(text) || !isIdentByte(text[end])
		if leftOK && rightOK {
			return true
		}
		from = pos + 1
	}
	return false
}

func isIdentByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_':
		return true
	}
	return false
}

// UniqueTrimmed trims each value, drops empties and duplicates, and keeps first-seen
// order. It reuses values' backing array and returns nil for empty input.
func UniqueTrimmed(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
