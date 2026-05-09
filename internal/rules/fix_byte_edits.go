package rules

import (
	"sort"
	"strings"
)

// byteEdit describes a single in-range substitution: the bytes
// [start,end) are replaced with repl. Used by autofix builders that
// rewrite a contiguous AST node by composing several point edits.
type byteEdit struct {
	start, end int
	repl       string
}

// applyByteEdits applies the given edits to content[rangeStart:rangeEnd]
// and returns the rewritten string. Edits are sorted by start byte
// before application; callers can supply them in any order.
//
// Returns ok=false when any edit is outside the range, edits overlap, or
// edits are out of order after sorting — i.e. anything that would
// produce malformed output. Callers should treat that as "skip the fix"
// and emit the finding without one.
func applyByteEdits(content []byte, rangeStart, rangeEnd int, edits []byteEdit) (string, bool) {
	if rangeStart < 0 || rangeEnd > len(content) || rangeStart > rangeEnd {
		return "", false
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var b strings.Builder
	cursor := rangeStart
	for _, e := range edits {
		if e.start < cursor || e.end > rangeEnd || e.start > e.end {
			return "", false
		}
		b.Write(content[cursor:e.start])
		b.WriteString(e.repl)
		cursor = e.end
	}
	b.Write(content[cursor:rangeEnd])
	return b.String(), true
}
