package api

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// fillSpan sets StartByte/EndByte on a finding that carries only a line and
// column, so every emitted finding has a byte span. The start is the reported
// position; the end is the end of the innermost named syntax node that starts
// there (the token or identifier at the position), or the end of the token at
// that position when the file has no syntax tree. Rules that know the whole
// flagged node report its full span through EmitAtNode.
func (c *Context) fillSpan(f *scanner.Finding) {
	if f.EndByte > f.StartByte {
		return
	}
	if f.Line < 1 || f.Col < 1 {
		return
	}
	file := c.File
	if file == nil || f.File != file.Path || len(file.Content) == 0 {
		// Resource and manifest findings carry a path without parsed source content.
		if strings.HasSuffix(f.File, ".xml") {
			f.StartByte, f.EndByte = xmlSpan(f.File, f.Line, f.Col)
		}
		return
	}
	start := file.LineOffset(f.Line-1) + f.Col - 1
	if start < 0 || start >= len(file.Content) {
		return
	}
	end := operatorTokenEnd(file.Content, start)
	if end <= start {
		end = nodeEndAt(file, start)
	}
	if end <= start {
		end = tokenEndAt(file.Content, start)
	}
	if end <= start {
		return
	}
	f.StartByte, f.EndByte = start, end
}

// spanOperators are postfix and infix operators whose finding span is the token
// itself; the enclosing named node (a navigation suffix) or the blank-delimited
// run would otherwise stretch the span over the rest of the call chain.
var spanOperators = []string{"!!", "?.", "?:"}

// operatorTokenEnd returns the end of the operator token at start, or 0 when
// the bytes there are not one of spanOperators.
func operatorTokenEnd(content []byte, start int) int {
	for _, op := range spanOperators {
		if start+len(op) <= len(content) && string(content[start:start+len(op)]) == op {
			return start + len(op)
		}
	}
	return 0
}

// nodeEndAt returns the end byte of the innermost named node starting exactly
// at start, or 0 when the file has no tree or no named node starts there.
func nodeEndAt(file *scanner.File, start int) int {
	t := file.FlatTree
	if t == nil || len(t.Types) == 0 {
		return 0
	}
	// Start offsets never decrease in node order, so binary search for the first node at start.
	lo, hi := 0, len(t.StartBytes)
	for lo < hi {
		mid := (lo + hi) / 2
		if int(t.StartBytes[mid]) < start {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	end := 0
	for i := lo; i < len(t.StartBytes) && int(t.StartBytes[i]) == start; i++ {
		e := int(t.EndBytes[i])
		if e > start && file.FlatIsNamed(uint32(i)) && (end == 0 || e < end) {
			end = e
		}
	}
	return end
}

// tokenEndAt returns the end of the token at start: a quoted attribute value
// when the position is an attribute name, else the run of non-blank bytes.
func tokenEndAt(content []byte, start int) int {
	i := start
	for i < len(content) && !isSpanBreak(content[i]) && content[i] != '=' {
		i++
	}
	if i+1 < len(content) && content[i] == '=' && (content[i+1] == '"' || content[i+1] == '\'') {
		quote := content[i+1]
		for j := i + 2; j < len(content) && content[j] != '\n'; j++ {
			if content[j] == quote {
				return j + 1
			}
		}
	}
	if i == start {
		for i < len(content) && !isSpanBreak(content[i]) {
			i++
		}
	}
	return i
}

func isSpanBreak(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '>'
}
