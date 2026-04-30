package rules

import (
	"fmt"
	"strings"
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

type complexityMetrics struct {
	maxNestedDepth       int
	deepestLine          int
	cyclomatic           int
	cyclomaticSimpleSkip int // cyclomatic with simple when_entry nodes excluded
	cognitive            int
}

type complexityMetricsKey struct {
	filePath string
	start    int
	end      int
	nodeType string
}

var complexityMetricsCache sync.Map

func getComplexityMetricsFlat(idx uint32, file *scanner.File) complexityMetrics {
	if file == nil {
		return complexityMetrics{}
	}
	key := complexityMetricsKey{
		filePath: file.Path,
		start:    int(file.FlatStartByte(idx)),
		end:      int(file.FlatEndByte(idx)),
		nodeType: file.FlatType(idx),
	}
	if cached, ok := complexityMetricsCache.Load(key); ok {
		return cached.(complexityMetrics)
	}
	m := collectComplexityMetricsFlat(idx, file)
	complexityMetricsCache.Store(key, m)
	return m
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
			if !(nodeType == "when_entry" && isSimpleWhenEntryFlat(file, idx)) {
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
		for child := file.FlatTree.Nodes[idx].FirstChild; child != 0; child = file.FlatTree.Nodes[child].NextSib {
			if !file.FlatTree.Nodes[child].IsNamed() {
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
	for c := file.FlatTree.Nodes[p].FirstChild; c != 0; c = file.FlatTree.Nodes[c].NextSib {
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

// LongMethodRule detects functions exceeding a line count.
type LongMethodRule struct {
	FlatDispatchBase
	BaseRule
	AllowedLines int
}

// Description implements DescriptionProvider.
func (*LongMethodRule) Description() string {
	return "Flags functions that exceed the configured line limit. Long functions are harder to test and understand — consider extracting logical sub-steps into named helpers."
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *LongMethodRule) Confidence() float64 { return 0.75 }

// longMethodDeclarationLineFlat returns the 1-based line number of the `fun`
// keyword within a function_declaration node. It walks the node's direct
// children looking for the anonymous "fun" token, which is structurally
// reliable for annotated functions (e.g. @Override\nsuspend fun) where the
// node's start row is the annotation row, not the fun keyword row.
func longMethodDeclarationLineFlat(file *scanner.File, idx uint32) int {
	if file == nil {
		return 1
	}
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatNodeTextEquals(c, "fun") {
			return file.FlatRow(c) + 1
		}
	}
	return file.FlatRow(idx) + 1
}

// countSignificantLines returns the number of non-blank, non-comment lines
// in the inclusive row range [startRow, endRow]. This matches detekt's line
// counting semantics for LongMethod/LargeClass rules: blank lines and
// comment-only lines are excluded, as are lines fully inside a triple-quoted
// string literal (which are string content, not source lines).
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
			if !strings.Contains(trimmed[2:], "*/") {
				// Stays in block comment; already reflected in state.
			}
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
	i := 0
	n := len(line)
	inLineComment := false
	inRegString := false
	for i < n {
		if inLineComment {
			return
		}
		if st.inBlockComment {
			if i+1 < n && line[i] == '*' && line[i+1] == '/' {
				st.inBlockComment = false
				i += 2
				continue
			}
			i++
			continue
		}
		if st.inRawString {
			if i+2 < n && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
				st.inRawString = false
				i += 3
				continue
			}
			i++
			continue
		}
		if inRegString {
			if line[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if line[i] == '"' {
				inRegString = false
				i++
				continue
			}
			i++
			continue
		}
		// Code position.
		if i+1 < n && line[i] == '/' && line[i+1] == '/' {
			inLineComment = true
			i += 2
			continue
		}
		if i+1 < n && line[i] == '/' && line[i+1] == '*' {
			st.inBlockComment = true
			i += 2
			continue
		}
		if i+2 < n && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
			st.inRawString = true
			i += 3
			continue
		}
		if line[i] == '"' {
			inRegString = true
			i++
			continue
		}
		i++
	}
}

func flatEndRow(file *scanner.File, idx uint32) int {
	return file.FlatRow(idx) + strings.Count(file.FlatNodeText(idx), "\n")
}

// LargeClassRule detects classes exceeding a line count.
type LargeClassRule struct {
	FlatDispatchBase
	BaseRule
	AllowedLines int
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *LargeClassRule) Confidence() float64 { return 0.75 }


// NestedBlockDepthRule detects excessive nesting.
type NestedBlockDepthRule struct {
	FlatDispatchBase
	BaseRule
	AllowedDepth int
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *NestedBlockDepthRule) Confidence() float64 { return 0.75 }


// nestedBlockDepthExceedsFlat returns the function's max nesting depth
// (read from the shared complexityMetricsCache populated once per
// function per file), the line where the deepest nesting occurs, and
// whether the depth exceeds the allowed threshold.
//
// Historically this function had its own walker with early-exit at the
// threshold; it now routes through getComplexityMetricsFlat so
// CyclomaticComplexMethod, NestedBlockDepth, and CognitiveComplexMethod
// share a single per-function walk. Line semantics are now "where max
// depth was reached" instead of "where threshold was first crossed,"
// which is arguably more useful to rule consumers.
func nestedBlockDepthExceedsFlat(file *scanner.File, root uint32, allowed int) (depth int, line int, exceeded bool) {
	if file == nil || file.FlatTree == nil {
		return 0, 0, false
	}
	metrics := getComplexityMetricsFlat(root, file)
	depth = metrics.maxNestedDepth
	if depth > allowed {
		line = metrics.deepestLine
		exceeded = true
	}
	return depth, line, exceeded
}

// CyclomaticComplexMethodRule counts decision points per function.
type CyclomaticComplexMethodRule struct {
	FlatDispatchBase
	BaseRule
	AllowedComplexity          int
	IgnoreSimpleWhenEntries    bool
	IgnoreSingleWhenExpression bool
	IgnoreLocalFunctions       bool
	IgnoreNestingFunctions     bool
	NestingFunctions           []string
}

// Description implements DescriptionProvider.
func (*CyclomaticComplexMethodRule) Description() string {
	return "Counts independent paths through a function (branches, loops, catches). High cyclomatic complexity predicts defect density and makes code harder to test exhaustively."
}

var decisionTypes = map[string]bool{
	"if_expression":          true,
	"for_statement":          true,
	"while_statement":        true,
	"do_while_statement":     true,
	"catch_block":            true,
	"when_entry":             true,
	"conjunction_expression": true,
	"disjunction_expression": true,
}

func isCyclomaticDecisionType(nodeType string) bool {
	return decisionTypes[nodeType]
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *CyclomaticComplexMethodRule) Confidence() float64 { return 0.75 }


func isPureBooleanPredicateFlat(file *scanner.File, fn uint32) bool {
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return false
	}
	bodyText := strings.TrimSpace(file.FlatNodeText(body))
	if strings.HasPrefix(bodyText, "=") {
		for i := 0; i < file.FlatNamedChildCount(body); i++ {
			c := file.FlatNamedChild(body, i)
			if t := file.FlatType(c); t == "disjunction_expression" || t == "conjunction_expression" {
				return !containsDeepControlFlowFlat(file, c)
			}
		}
		return false
	}
	stmts, _ := file.FlatFindChild(body, "statements")
	if stmts == 0 || file.FlatNamedChildCount(stmts) != 1 {
		return false
	}
	stmt := file.FlatNamedChild(stmts, 0)
	if file.FlatType(stmt) != "jump_expression" {
		return false
	}
	for i := 0; i < file.FlatNamedChildCount(stmt); i++ {
		c := file.FlatNamedChild(stmt, i)
		if t := file.FlatType(c); t == "disjunction_expression" || t == "conjunction_expression" {
			return !containsDeepControlFlowFlat(file, c)
		}
	}
	return false
}

func containsDeepControlFlowFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "if_expression", "when_expression", "try_expression", "for_statement", "while_statement", "do_while_statement":
		return true
	case "lambda_literal", "function_declaration":
		return false
	}
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		if containsDeepControlFlowFlat(file, file.FlatNamedChild(idx, i)) {
			return true
		}
	}
	return false
}

func isSimpleWhenEntryFlat(file *scanner.File, entry uint32) bool {
	for i := 0; i < file.FlatChildCount(entry); i++ {
		child := file.FlatChild(entry, i)
		if file.FlatType(child) == "control_structure_body" {
			stmts, _ := file.FlatFindChild(child, "statements")
			if stmts == 0 {
				return true
			}
			return file.FlatNamedChildCount(stmts) <= 1
		}
	}
	return true
}

// cyclomaticComplexityExceedsFlat returns the function's cyclomatic
// complexity (read from the shared complexityMetricsCache populated once
// per function per file), the line to associate with a finding, and
// whether the complexity exceeds the allowed threshold.
//
// Historically this had its own walker with early-exit; it now routes
// through getComplexityMetricsFlat so all three complexity rules share
// one walk per function. The IgnoreSimpleWhenEntries flag picks between
// the two cyclomatic variants computed simultaneously in that walk. The
// reported line is the function's start line (the shared walker doesn't
// track a threshold-crossing line since it doesn't early-exit).
func cyclomaticComplexityExceedsFlat(file *scanner.File, root uint32, allowed int, ignoreSimpleWhenEntries bool) (complexity int, line int, exceeded bool) {
	if file == nil || file.FlatTree == nil {
		return 0, 0, false
	}
	metrics := getComplexityMetricsFlat(root, file)
	if ignoreSimpleWhenEntries {
		complexity = metrics.cyclomaticSimpleSkip
	} else {
		complexity = metrics.cyclomatic
	}
	if complexity > allowed {
		line = file.FlatRow(root) + 1
		exceeded = true
	}
	return complexity, line, exceeded
}

// CognitiveComplexMethodRule measures cognitive complexity, weighting nesting depth.
type CognitiveComplexMethodRule struct {
	FlatDispatchBase
	BaseRule
	AllowedComplexity int
}

var cognitiveTypes = map[string]bool{
	"if_expression":      true,
	"for_statement":      true,
	"while_statement":    true,
	"do_while_statement": true,
	"when_expression":    true,
	"catch_block":        true,
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *CognitiveComplexMethodRule) Confidence() float64 { return 0.75 }


// ComplexConditionRule detects conditions with too many logical operators.
type ComplexConditionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedConditions int
}

func countLogicalOperatorsOutsideBodiesFlat(file *scanner.File, root uint32) int {
	count := 0
	var walk func(uint32, bool)
	walk = func(idx uint32, inBody bool) {
		if idx != root {
			switch file.FlatType(idx) {
			case "function_declaration", "lambda_literal", "anonymous_function":
				return
			}
		}
		switch file.FlatType(idx) {
		case "control_structure_body", "statements":
			inBody = true
		case "conjunction_expression", "disjunction_expression":
			if !inBody {
				count++
			}
		}
		for i := 0; i < file.FlatChildCount(idx); i++ {
			walk(file.FlatChild(idx, i), inBody)
		}
	}
	walk(root, false)
	return count
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *ComplexConditionRule) Confidence() float64 { return 0.75 }

func isPureDisjunctionOrConjunctionFlat(file *scanner.File, root uint32) bool {
	hasConj := false
	hasDisj := false
	var walk func(uint32, bool)
	walk = func(idx uint32, inBody bool) {
		if idx != root {
			switch file.FlatType(idx) {
			case "function_declaration", "lambda_literal", "anonymous_function":
				return
			}
		}
		switch file.FlatType(idx) {
		case "control_structure_body", "statements":
			inBody = true
		case "conjunction_expression":
			if !inBody {
				hasConj = true
			}
		case "disjunction_expression":
			if !inBody {
				hasDisj = true
			}
		}
		for i := 0; i < file.FlatChildCount(idx); i++ {
			walk(file.FlatChild(idx, i), inBody)
		}
	}
	walk(root, false)
	return !(hasConj && hasDisj)
}

// ComplexInterfaceRule detects interfaces with too many members.
type ComplexInterfaceRule struct {
	FlatDispatchBase
	BaseRule
	AllowedDefinitions         int
	IncludePrivateDeclarations bool
	IncludeStaticDeclarations  bool
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *ComplexInterfaceRule) Confidence() float64 { return 0.75 }


// LabeledExpressionRule detects return@label, break@label, continue@label.
type LabeledExpressionRule struct {
	FlatDispatchBase
	BaseRule
	IgnoredLabels []string
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *LabeledExpressionRule) Confidence() float64 { return 0.75 }

// labeledExpressionLabelIgnored reports whether the label-node text matches
// any name in the ignored list. Tree-sitter Kotlin emits a `label` node for
// the trailing-`@` form (`process@`) used at the definition site; the
// comparison strips the `@` so users configure plain identifiers like
// `forEach` or `process` (matching detekt's `ignoredLabels` semantics).
func labeledExpressionLabelIgnored(labelText string, ignored []string) bool {
	if len(ignored) == 0 {
		return false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(labelText, "@"), "@")
	if name == "" {
		return false
	}
	for _, candidate := range ignored {
		if candidate == name {
			return true
		}
	}
	return false
}

// LongParameterListRule detects functions/constructors with too many parameters.
type LongParameterListRule struct {
	FlatDispatchBase
	BaseRule
	AllowedFunctionParameters    int
	AllowedConstructorParameters int
	IgnoreDefaultParameters      bool     // if true, parameters with defaults are not counted
	IgnoreDataClasses            bool     // if true, data class constructors are skipped
	IgnoreAnnotatedParameter     []string // annotations that exclude a parameter from counting
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *LongParameterListRule) Confidence() float64 { return 0.75 }

// paramHasIgnoredAnnotationFlat reports whether the parameter at idx
// carries any annotation whose simple name matches one of the names in
// `annotations`. Used to honor LongParameterListRule.IgnoreAnnotatedParameter
// so that, e.g., `@Composable`-annotated parameters can be excluded from
// the parameter count.
//
// Tree-sitter Kotlin attaches annotations on function parameters to a
// `parameter_modifiers` sibling that precedes the `parameter` node, while
// class-constructor parameters carry their annotations as children of the
// `class_parameter` node. The helper consults both shapes so callers can
// pass either node kind opaquely.
func paramHasIgnoredAnnotationFlat(file *scanner.File, idx uint32, annotations []string) bool {
	if len(annotations) == 0 || file == nil || idx == 0 {
		return false
	}
	// 1) Annotations attached as children (class_parameter case, and any
	//    parameter shape that nests a `modifiers` child).
	for _, name := range annotations {
		if name == "" {
			continue
		}
		if flatHasAnnotationNamed(file, idx, name) {
			return true
		}
	}
	// 2) Function-parameter case: annotations live on the preceding
	//    `parameter_modifiers` sibling within `function_value_parameters`.
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	var prevModifiers uint32
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if child == idx {
			break
		}
		if file.FlatType(child) == "parameter_modifiers" {
			prevModifiers = child
		} else if file.FlatType(child) == "parameter" || file.FlatType(child) == "class_parameter" {
			// Only the parameter_modifiers immediately preceding `idx`
			// belongs to it; reset on every other parameter node.
			prevModifiers = 0
		}
	}
	if prevModifiers == 0 {
		return false
	}
	for _, name := range annotations {
		if name == "" {
			continue
		}
		if flatAnnotationListContains("", file.FlatNodeText(prevModifiers), name) {
			return true
		}
	}
	return false
}

// MethodOverloadingRule detects too many overloads of the same method name.
type MethodOverloadingRule struct {
	FlatDispatchBase
	BaseRule
	AllowedOverloads int
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *MethodOverloadingRule) Confidence() float64 { return 0.75 }

func (r *MethodOverloadingRule) checkScopeFlat(ctx *v2.Context, node uint32) {
	file := ctx.File
	counts := make(map[string]int)
	firstLine := make(map[string]int)
	addFunction := func(fn uint32) {
		name := extractIdentifierFlat(file, fn)
		if name == "" {
			return
		}
		counts[name]++
		if _, ok := firstLine[name]; !ok {
			firstLine[name] = file.FlatRow(fn) + 1
		}
	}

	switch file.FlatType(node) {
	case "source_file":
		for i := 0; i < file.FlatChildCount(node); i++ {
			child := file.FlatChild(node, i)
			switch file.FlatType(child) {
			case "function_declaration":
				addFunction(child)
			case "class_declaration":
				r.checkScopeFlat(ctx, child)
			}
		}
	case "class_declaration":
		body, _ := file.FlatFindChild(node, "class_body")
		if body == 0 {
			return
		}
		forEachDirectClassBodyNodeFlat(file, body, func(child uint32) {
			switch file.FlatType(child) {
			case "function_declaration":
				addFunction(child)
			case "class_declaration":
				r.checkScopeFlat(ctx, child)
			}
		})
	default:
		return
	}
	for name, count := range counts {
		if count > r.AllowedOverloads {
			ctx.Emit(r.Finding(file, firstLine[name], 1,
				fmt.Sprintf("Method '%s' has %d overloads (allowed: %d).", name, count, r.AllowedOverloads)))
		}
	}
}

// NamedArgumentsRule detects function calls with too many unnamed arguments.
type NamedArgumentsRule struct {
	FlatDispatchBase
	BaseRule
	AllowedArguments int
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *NamedArgumentsRule) Confidence() float64 { return 0.75 }


// NestedScopeFunctionsRule detects nested scope functions (apply, also, let, run, with).
type NestedScopeFunctionsRule struct {
	FlatDispatchBase
	BaseRule
	AllowedDepth int
	Functions    []string
}

var scopeFunctions = map[string]bool{
	"apply": true, "also": true, "let": true, "run": true, "with": true,
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *NestedScopeFunctionsRule) Confidence() float64 { return 0.75 }

func extractCallNameFlat(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "simple_identifier":
			return file.FlatNodeText(child)
		case "navigation_expression":
			for j := file.FlatChildCount(child) - 1; j >= 0; j-- {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "navigation_suffix" {
					for k := 0; k < file.FlatChildCount(gc); k++ {
						ggc := file.FlatChild(gc, k)
						if file.FlatType(ggc) == "simple_identifier" {
							return file.FlatNodeText(ggc)
						}
					}
				}
				if file.FlatType(gc) == "simple_identifier" {
					return file.FlatNodeText(gc)
				}
			}
			return ""
		}
	}
	return ""
}

// ReplaceSafeCallChainWithRunRule detects 3+ chained safe calls (?.).
type ReplaceSafeCallChainWithRunRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *ReplaceSafeCallChainWithRunRule) Confidence() float64 { return 0.75 }

func countSafeCallsInChainFlat(file *scanner.File, idx uint32) int {
	if file.FlatType(idx) != "navigation_expression" {
		return 0
	}
	count := 0
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "navigation_suffix" && strings.HasPrefix(file.FlatNodeText(child), "?.") {
			count++
		}
	}
	if file.FlatChildCount(idx) == 0 {
		return count
	}
	receiver := file.FlatChild(idx, 0)
	switch file.FlatType(receiver) {
	case "navigation_expression":
		count += countSafeCallsInChainFlat(file, receiver)
	case "call_expression":
		if file.FlatChildCount(receiver) > 0 {
			inner := file.FlatChild(receiver, 0)
			if file.FlatType(inner) == "navigation_expression" {
				count += countSafeCallsInChainFlat(file, inner)
			}
		}
	}
	return count
}

// StringLiteralDuplicationRule detects duplicate string literals in a file.
type StringLiteralDuplicationRule struct {
	FlatDispatchBase
	BaseRule
	AllowedDuplications       int
	AllowedWithLengthLessThan int
	IgnoreAnnotation          bool
	IgnoreStringsRegex        string
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *StringLiteralDuplicationRule) Confidence() float64 { return 0.75 }

// stringLiteralUnquote strips the surrounding quote delimiters from a
// Kotlin string literal's raw text so that user-supplied
// IgnoreStringsRegex patterns match against the contents rather than
// the quoted form. Both regular ("...") and triple-quoted ("""...""")
// shapes are handled.
func stringLiteralUnquote(text string) string {
	if strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`) && len(text) >= 6 {
		return text[3 : len(text)-3]
	}
	if strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) && len(text) >= 2 {
		return text[1 : len(text)-1]
	}
	return text
}

// TooManyFunctionsRule detects files or classes with too many functions.
type TooManyFunctionsRule struct {
	FlatDispatchBase
	BaseRule
	AllowedFunctionsPerFile      int
	AllowedFunctionsPerClass     int
	AllowedFunctionsPerInterface int
	AllowedFunctionsPerObject    int
	AllowedFunctionsPerEnum      int
	IgnorePrivate                bool
	IgnoreDeprecated             bool
	IgnoreInternal               bool
	IgnoreOverridden             bool
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *TooManyFunctionsRule) Confidence() float64 { return 0.75 }

func (r *TooManyFunctionsRule) shouldCountFunctionFlat(fnNode uint32, file *scanner.File) bool {
	if r.IgnorePrivate && file.FlatHasModifier(fnNode, "private") {
		return false
	}
	if r.IgnoreInternal && file.FlatHasModifier(fnNode, "internal") {
		return false
	}
	if r.IgnoreOverridden && file.FlatHasModifier(fnNode, "override") {
		return false
	}
	if r.IgnoreDeprecated {
		if mods, ok := file.FlatFindChild(fnNode, "modifiers"); ok && strings.Contains(file.FlatNodeText(mods), "Deprecated") {
			return false
		}
	}
	return true
}


func (r *TooManyFunctionsRule) countFunctionsInClassFlat(cls uint32, file *scanner.File) int {
	body, _ := file.FlatFindChild(cls, "class_body")
	if body == 0 {
		return 0
	}
	count := 0
	forEachDirectClassBodyNodeFlat(file, body, func(child uint32) {
		if file.FlatType(child) == "function_declaration" && r.shouldCountFunctionFlat(child, file) {
			count++
		}
	})
	return count
}

func forEachDirectClassBodyNodeFlat(file *scanner.File, body uint32, fn func(uint32)) {
	if body == 0 || fn == nil {
		return
	}
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		switch file.FlatType(child) {
		case "function_declaration", "property_declaration", "class_declaration":
			fn(child)
		case "class_member_declarations":
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				switch file.FlatType(gc) {
				case "function_declaration", "property_declaration", "class_declaration":
					fn(gc)
				}
			}
		}
	}
}

func countDirectClassMembersFlat(file *scanner.File, body uint32) int {
	members := 0
	forEachDirectClassBodyNodeFlat(file, body, func(child uint32) {
		if t := file.FlatType(child); t == "function_declaration" || t == "property_declaration" {
			members++
		}
	})
	return members
}
