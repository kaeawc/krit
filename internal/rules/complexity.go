package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/scanner"
)

type complexityMetrics struct {
	maxNestedDepth int
	cyclomatic     int
	cognitive      int
}

type FunctionComplexity struct {
	Cognitive  int
	Cyclomatic int
}

func FunctionComplexities(file *scanner.File) []FunctionComplexity {
	if file == nil || file.FlatTree == nil {
		return nil
	}
	out := make([]FunctionComplexity, 0)
	for idx := uint32(1); idx < uint32(file.FlatTree.Len()); idx++ {
		if file.FlatType(idx) != "function_declaration" {
			continue
		}
		metrics := getComplexityMetricsFlat(idx, file)
		out = append(out, FunctionComplexity{
			Cognitive:  metrics.cognitive,
			Cyclomatic: metrics.cyclomatic,
		})
	}
	return out
}

func getComplexityMetricsFlat(idx uint32, file *scanner.File) complexityMetrics {
	if file == nil {
		return complexityMetrics{}
	}
	return filefacts.NodeFact(fileFactsCache(), file, idx, slotComplexity, func() complexityMetrics {
		return collectComplexityMetricsFlat(idx, file)
	})
}

// collectComplexityMetricsFlat measures a function with detekt's default settings, for reports that show
// complexity without applying a rule threshold.
func collectComplexityMetricsFlat(root uint32, file *scanner.File) complexityMetrics {
	if file == nil || file.FlatTree == nil {
		return complexityMetrics{}
	}
	return complexityMetrics{
		maxNestedDepth: detektNestedBlockDepth(file, root),
		cyclomatic:     detektCyclomaticComplexity(file, root, defaultCyclomaticOptions()),
		cognitive:      detektCognitiveComplexity(file, root),
	}
}

func isElseIfChainNodeFlat(file *scanner.File, idx uint32) bool {
	if file.FlatType(idx) != "if_expression" {
		return false
	}
	target := idx
	p, ok := file.FlatParent(idx)
	if ok && file.FlatType(p) == "control_structure_body" {
		target = p
		p, ok = file.FlatParent(p)
	}
	if !ok || file.FlatType(p) != "if_expression" {
		return false
	}
	// Walk parent's children: find the `else` keyword, then check whether
	// `target` is a later sibling. Sibling order is robust against
	// comments, error nodes, or other extras between `else` and the inner
	// `if` — byte offsets are not.
	sawElse := false
	t := file.FlatTree
	for c := t.FirstChildren[p]; c != 0; c = t.NextSibs[c] {
		if !sawElse {
			if file.FlatType(c) == "else" {
				sawElse = true
			}
			continue
		}
		if c == target {
			return true
		}
	}
	return false
}

func isNestingType(nodeType string) bool {
	switch nodeType {
	case "if_expression", "for_statement", "while_statement", "do_while_statement", "when_expression", "try_expression":
		return true
	default:
		return false
	}
}

// countSignificantLines returns the number of lines in the inclusive row range [startRow, endRow] that hold
// code. Blank lines and comment-only lines are excluded; a line with code and a comment counts. Non-blank
// lines inside a triple-quoted string count, as detekt counts them.
func countSignificantLines(file *scanner.File, startRow, endRow int) int {
	if startRow < 0 {
		startRow = 0
	}
	if endRow >= len(file.Lines) {
		endRow = len(file.Lines) - 1
	}
	count := 0
	state := lineScanState{}
	for i := startRow; i <= endRow; i++ {
		if strings.TrimSpace(stripCommentsKeepStrings(file.Lines[i], &state)) != "" {
			count++
		}
	}
	return count
}

// lineScanState tracks lexical state across lines for countSignificantLines.
type lineScanState struct {
	inBlockComment bool
	inRawString    bool
}

// scanLineState advances the line-scan state by walking `line` character by
// character, properly skipping content inside line comments, block comments,
// regular `"..."` strings (with `\"` escapes), and raw `"""..."""` strings.
// Only `"""` delimiters that appear in actual code position toggle the
// raw-string state — occurrences inside comments or regular strings are
// ignored.
func scanLineState(line string, st *lineScanState) {
	scanLineLexical(line, st, nil, true)
}

// stripCommentsAndRawStrings returns a copy of `line` where every byte
// inside a line comment, block comment, or raw `"""..."""` string
// literal is replaced with a single space. Regular `"..."` string
// literals are preserved verbatim — they typically hold real call
// arguments that line-based rules want to inspect, and Kotlin's
// requirement to escape inner `"` characters makes it hard for a
// regular string's body to fabricate a false positive for patterns
// that need a `"` boundary.
//
// The returned string has the same length as the input so column
// positions are stable; callers can run regex matches against the
// scrubbed line and reject hits that originated in non-code regions
// without losing positional information.
//
// `st` carries multi-line block-comment and raw-string state across
// calls; callers iterating over `file.Lines` should declare one
// lineScanState outside the loop and reuse it.
func stripCommentsAndRawStrings(line string, st *lineScanState) string {
	if line == "" {
		return ""
	}
	// Fast path: when no multi-line construct is open and the line cannot
	// start one (no `//`, `/*`, or `"""`), the scrub is a no-op and state
	// stays clean. Regular `"..."` strings are preserved verbatim and do
	// not carry state across lines, so we don't need to detect them here.
	if !st.inBlockComment && !st.inRawString &&
		!strings.Contains(line, "//") &&
		!strings.Contains(line, "/*") &&
		!strings.Contains(line, `"""`) {
		return line
	}
	out := []byte(line)
	scanLineLexical(line, st, out, true)
	return string(out)
}

// stripCommentsKeepStrings is the sibling of stripCommentsAndRawStrings
// for rules whose patterns may legitimately appear inside both regular
// and raw string literals (e.g. a private-key marker pasted into a
// triple-quoted string). It masks only line-comment and block-comment
// bytes; regular and raw string bytes are preserved verbatim. Length
// preservation and state semantics match the other helper.
func stripCommentsKeepStrings(line string, st *lineScanState) string {
	if line == "" {
		return ""
	}
	if !st.inBlockComment &&
		!strings.Contains(line, "//") &&
		!strings.Contains(line, "/*") {
		// Lex still has to advance for the in-line content (raw and
		// regular string toggles), so any raw string that opens on this
		// line is recorded for the next call. No bytes are masked.
		scanLineLexical(line, st, nil, false)
		return line
	}
	out := []byte(line)
	scanLineLexical(line, st, out, false)
	return string(out)
}

// scanLineLexical is the shared body of scanLineState and the
// stripComments* helpers. It advances `st` across `line` and, when
// `mask` is non-nil, overwrites every byte that falls inside a line or
// block comment with a space. Raw `"""..."""` strings are also masked
// when `maskRaw` is true; otherwise their bytes are preserved (the lex
// still advances `st.inRawString`). Regular `"..."` strings advance
// the lexer's state (so `\"` and inner `"` are handled correctly) but
// their bytes are never masked.
func scanLineLexical(line string, st *lineScanState, mask []byte, maskRaw bool) {
	n := len(line)
	inLineComment := false
	inRegString := false
	for i := 0; i < n; {
		switch {
		case inLineComment:
			if mask == nil {
				return
			}
			mask[i] = ' '
			i++
		case st.inBlockComment:
			i = lexAdvanceBlockComment(line, i, n, st, mask)
		case st.inRawString:
			i = lexAdvanceRawString(line, i, n, st, mask, maskRaw)
		case inRegString:
			i, inRegString = lexAdvanceRegString(line, i, n)
		default:
			i, inLineComment, inRegString = lexAdvanceCodePos(
				line, i, n, st, mask, maskRaw, inRegString)
		}
	}
}

// lexAdvanceBlockComment masks the current byte (and the closing `*/`
// when present) and returns the next index. Caller ensures we are
// inside a block comment.
func lexAdvanceBlockComment(line string, i, n int, st *lineScanState, mask []byte) int {
	if i+1 < n && line[i] == '*' && line[i+1] == '/' {
		maskByte(mask, i)
		maskByte(mask, i+1)
		st.inBlockComment = false
		return i + 2
	}
	maskByte(mask, i)
	return i + 1
}

// lexAdvanceRawString masks (when maskRaw is true) the current raw-
// string byte and the closing `"""` when present. Caller ensures we
// are inside a raw string.
func lexAdvanceRawString(line string, i, n int, st *lineScanState, mask []byte, maskRaw bool) int {
	if i+2 < n && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
		if maskRaw {
			maskByte(mask, i)
			maskByte(mask, i+1)
			maskByte(mask, i+2)
		}
		st.inRawString = false
		return i + 3
	}
	if maskRaw {
		maskByte(mask, i)
	}
	return i + 1
}

// lexAdvanceRegString advances past one byte inside a regular `"..."`
// string, honouring `\"` escapes and the closing `"`.
func lexAdvanceRegString(line string, i, n int) (int, bool) {
	if line[i] == '\\' && i+1 < n {
		return i + 2, true
	}
	if line[i] == '"' {
		return i + 1, false
	}
	return i + 1, true
}

// lexAdvanceCodePos consumes one code-position byte and returns the
// new index plus the updated `inLineComment` and `inRegString` flags.
// State transitions (`//`, `/*`, `"""`, `"`) are recognized here.
func lexAdvanceCodePos(line string, i, n int, st *lineScanState, mask []byte, maskRaw, inRegString bool) (int, bool, bool) {
	if i+1 < n && line[i] == '/' && line[i+1] == '/' {
		maskByte(mask, i)
		maskByte(mask, i+1)
		return i + 2, true, inRegString
	}
	if i+1 < n && line[i] == '/' && line[i+1] == '*' {
		st.inBlockComment = true
		maskByte(mask, i)
		maskByte(mask, i+1)
		return i + 2, false, inRegString
	}
	if i+2 < n && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
		st.inRawString = true
		if maskRaw {
			maskByte(mask, i)
			maskByte(mask, i+1)
			maskByte(mask, i+2)
		}
		return i + 3, false, inRegString
	}
	if line[i] == '"' {
		return i + 1, false, true
	}
	return i + 1, false, inRegString
}

func maskByte(mask []byte, i int) {
	if mask != nil {
		mask[i] = ' '
	}
}

func flatEndRow(file *scanner.File, idx uint32) int {
	return file.FlatRow(idx) + strings.Count(file.FlatNodeText(idx), "\n")
}
