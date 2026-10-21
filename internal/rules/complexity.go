package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/scanner"
)

type complexityMetrics struct {
	maxNestedDepth       int
	deepestLine          int
	cyclomatic           int
	cyclomaticSimpleSkip int // cyclomatic with simple when_entry nodes excluded
	cognitive            int
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

func collectComplexityMetricsFlat(root uint32, file *scanner.File) complexityMetrics {
	var m complexityMetrics
	if file == nil || file.FlatTree == nil {
		return m
	}
	m.cyclomatic = 1
	m.cyclomaticSimpleSkip = 1
	var walk func(uint32, int, int)
	walk = func(idx uint32, depthNesting int, cognitiveNesting int) {
		nodeType := file.FlatType(idx)
		if idx != root && (nodeType == "function_declaration" || nodeType == "lambda_literal") {
			return
		}
		nextDepthNesting := depthNesting
		if isNestingType(nodeType) {
			if !isElseIfChainNodeFlat(file, idx) {
				nextDepthNesting++
			}
			if nextDepthNesting > m.maxNestedDepth {
				m.maxNestedDepth = nextDepthNesting
				m.deepestLine = file.FlatRow(idx) + 1
			}
		}
		if decisionTypes[nodeType] {
			m.cyclomatic++
			// cyclomaticSimpleSkip excludes simple when_entry nodes (used by
			// CyclomaticComplexMethodRule when IgnoreSimpleWhenEntries is set).
			if nodeType != "when_entry" || !isSimpleWhenEntryFlat(file, idx) {
				m.cyclomaticSimpleSkip++
			}
		}
		if nodeType == "elvis_expression" {
			m.cyclomatic++
			m.cyclomaticSimpleSkip++
		}
		nextCognitiveNesting := cognitiveNesting
		if cognitiveTypes[nodeType] {
			m.cognitive += 1 + cognitiveNesting
			nextCognitiveNesting++
		}
		if nodeType == "conjunction_expression" || nodeType == "disjunction_expression" || nodeType == "elvis_expression" {
			m.cognitive++
		}
		// Iterate children directly via FirstChild/NextSib (O(N) total)
		// instead of FlatNamedChild(idx, i) in a loop (O(N²) due to O(i)
		// per FlatNamedChild call).
		t := file.FlatTree
		for child := t.FirstChildren[idx]; child != 0; child = t.NextSibs[child] {
			if !t.NodeIsNamed(child) {
				continue
			}
			walk(child, nextDepthNesting, nextCognitiveNesting)
		}
	}
	walk(root, 0, 0)
	return m
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
	case "if_expression", "for_statement", "while_statement", "do_while_statement", "when_expression":
		return true
	// try_expression intentionally NOT counted — try/catch/finally are
	// cleanup boundaries, not control-flow branching that adds cognitive
	// nesting depth.
	default:
		return false
	}
}

// androidLifecycleMethods are Android Fragment/Activity/View lifecycle
// callbacks that are often legitimately long due to wiring setup.
var androidLifecycleMethods = map[string]bool{
	"onCreate":               true,
	"onCreateView":           true,
	"onViewCreated":          true,
	"onAttach":               true,
	"onResume":               true,
	"onStart":                true,
	"onActivityCreated":      true,
	"onCreateDialog":         true,
	"onCreateOptionsMenu":    true,
	"onOptionsItemSelected":  true,
	"onConfigurationChanged": true,
	// View-wiring / fragment binding helpers — conventional names that
	// do the same work as onViewCreated.
	"bindAdapter": true,
	"bindView":    true,
	"bindData":    true,
	"bind":        true,
	"initViews":   true,
	"initView":    true,
	"setupViews":  true,
	"setupView":   true,
	"setupUi":     true,
	"setupUI":     true,
	"configure":   true,
	"configureUi": true,
	"configureUI": true,
}

func isDSLBuilderBodyFlat(idx uint32, file *scanner.File) bool {
	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return false
	}
	bodyText := strings.TrimSpace(file.FlatNodeText(body))
	if strings.HasPrefix(bodyText, "=") {
		return strings.Contains(bodyText, "{") && strings.HasSuffix(bodyText, "}")
	}
	stmts, _ := file.FlatFindChild(body, "statements")
	if stmts == 0 {
		return false
	}
	var namedChildren []uint32
	for i := 0; i < file.FlatNamedChildCount(stmts); i++ {
		namedChildren = append(namedChildren, file.FlatNamedChild(stmts, i))
	}
	if len(namedChildren) == 0 {
		return false
	}
	last := namedChildren[len(namedChildren)-1]
	checkNode := last
	if file.FlatType(checkNode) == "jump_expression" && file.FlatNamedChildCount(checkNode) > 0 {
		checkNode = file.FlatNamedChild(checkNode, 0)
	}
	if file.FlatType(checkNode) != "call_expression" {
		return false
	}
	suffix, _ := file.FlatFindChild(checkNode, "call_suffix")
	if suffix == 0 {
		return false
	}
	hasTrailingLambda := false
	for i := 0; i < file.FlatChildCount(suffix); i++ {
		t := file.FlatType(file.FlatChild(suffix, i))
		if t == "annotated_lambda" || t == "lambda_literal" {
			hasTrailingLambda = true
			break
		}
	}
	if !hasTrailingLambda {
		return false
	}
	for i := 0; i < len(namedChildren)-1; i++ {
		stmt := namedChildren[i]
		switch file.FlatType(stmt) {
		case "property_declaration", "variable_declaration", "assignment", "multi_variable_declaration":
		case "call_expression":
			if strings.Count(file.FlatNodeText(stmt), "\n") > 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// countSignificantLines returns the number of non-blank, non-comment lines
// in the inclusive row range [startRow, endRow]. Blank lines and comment-only
// lines are excluded, as are lines fully inside a triple-quoted string literal
// (which are string content, not source lines).
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
		line := file.Lines[i]
		trimmed := strings.TrimSpace(line)
		priorBlockComment := state.inBlockComment
		priorRawString := state.inRawString
		scanLineState(line, &state)
		// Entirely inside a raw string from start to end — string content,
		// not a source line.
		if priorRawString && state.inRawString {
			continue
		}
		// Entirely inside a block comment from start to end.
		if priorBlockComment && state.inBlockComment {
			continue
		}
		if priorBlockComment {
			// Block comment closed mid-line; if nothing else of substance
			// remains we still skip. A simple heuristic: if the line ends
			// with `*/` and has no further code, treat as comment-only.
			idx := strings.Index(line, "*/")
			if idx >= 0 {
				rest := strings.TrimSpace(line[idx+2:])
				if rest == "" {
					continue
				}
			}
			count++
			continue
		}
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "*") && !strings.HasPrefix(trimmed, "*=") {
			// KDoc continuation line.
			continue
		}
		if strings.HasPrefix(trimmed, "/*") {
			// If the block comment is not closed on this line, the caller
			// already updated inBlockComment state before reaching here.
			continue
		}
		count++
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
