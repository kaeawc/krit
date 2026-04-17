package rules

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

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
	p, ok := file.FlatParent(idx)
	if ok && file.FlatType(p) == "control_structure_body" {
		p, ok = file.FlatParent(p)
	}
	if !ok || file.FlatType(p) != "if_expression" {
		return false
	}
	elseEnd := -1
	for i := 0; i < file.FlatChildCount(p); i++ {
		c := file.FlatChild(p, i)
		if file.FlatType(c) == "else" {
			elseEnd = int(file.FlatEndByte(c))
			break
		}
	}
	return elseEnd >= 0 && int(file.FlatStartByte(idx)) >= elseEnd
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
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return false
	}
	bodyText := strings.TrimSpace(file.FlatNodeText(body))
	if strings.HasPrefix(bodyText, "=") {
		return strings.Contains(bodyText, "{") && strings.HasSuffix(bodyText, "}")
	}
	stmts := file.FlatFindChild(body, "statements")
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
	suffix := file.FlatFindChild(checkNode, "call_suffix")
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

func (r *LongMethodRule) NodeTypes() []string { return []string{"function_declaration"} }

var longMethodDeclKeywordRe = regexp.MustCompile(`(^|\s)fun\b`)

func (r *LongMethodRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip test files — test helpers/fixtures often have long setup bodies.
	if isTestFile(file.Path) {
		return nil
	}
	// Skip database table/DAO files — per-column SQL mapping is inherently
	// long and splitting into helpers fragments transactions.
	if strings.HasSuffix(file.Path, "Table.kt") || strings.HasSuffix(file.Path, "Tables.kt") ||
		strings.HasSuffix(file.Path, "Dao.kt") || strings.HasSuffix(file.Path, "DAO.kt") {
		return nil
	}
	// Skip @Composable functions — Jetpack Compose DSL is deeply nested and
	// line count is not a meaningful complexity metric for UI functions.
	if flatHasAnnotationNamed(file, idx, "Composable") {
		return nil
	}
	// Skip test functions — test bodies legitimately have long
	// Arrange/Act/Assert blocks.
	if flatHasAnnotationNamed(file, idx, "Test") ||
		flatHasAnnotationNamed(file, idx, "ParameterizedTest") {
		return nil
	}
	// Skip database migration files — schema migrations are inherently long.
	if strings.Contains(file.Path, "/migration/") ||
		strings.Contains(file.Path, "\\migration\\") ||
		strings.Contains(file.Path, "/migrations/") ||
		strings.Contains(file.Path, "\\migrations\\") {
		return nil
	}
	// Skip functions whose body is a single top-level call with a trailing
	// lambda — DSL builders like `configure { ... }`, `setContent { ... }`,
	// `buildList { ... }`, etc. Line count of a DSL body is not a meaningful
	// complexity metric.
	if isDSLBuilderBodyFlat(idx, file) {
		return nil
	}
	// Skip Android lifecycle wiring methods and common view-binding
	// helpers — these are cohesive init sequences where splitting into
	// helpers typically harms readability.
	name := extractIdentifierFlat(file, idx)
	if androidLifecycleMethods[name] {
		return nil
	}
	// Skip Signal Job.run()/onRun()/doRun() entry points — the Job
	// framework requires atomic execution, so splitting into helpers
	// typically fragments the transaction boundary without benefit.
	if strings.HasSuffix(file.Path, "Job.kt") &&
		(name == "run" || name == "onRun" || name == "doRun" || name == "onHandle") {
		return nil
	}
	lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx))
	if lines > r.AllowedLines {
		line := longMethodDeclarationLineFlat(file, idx)
		return []scanner.Finding{r.Finding(file, line, 1,
			fmt.Sprintf("Function '%s' has %d lines (allowed: %d).", name, lines, r.AllowedLines))}
	}
	return nil
}

func longMethodDeclarationLineFlat(file *scanner.File, idx uint32) int {
	if file == nil {
		return 1
	}
	startRow := file.FlatRow(idx)
	endRow := flatEndRow(file, idx)
	if startRow < 0 {
		startRow = 0
	}
	if endRow >= len(file.Lines) {
		endRow = len(file.Lines) - 1
	}
	for row := startRow; row <= endRow && row < len(file.Lines); row++ {
		if longMethodDeclKeywordRe.MatchString(strings.TrimSpace(file.Lines[row])) {
			return row + 1
		}
	}
	return startRow + 1
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
	inBlockComment := false
	inRawString := false
	for i := startRow; i <= endRow; i++ {
		line := file.Lines[i]
		trimmed := strings.TrimSpace(line)
		// Track triple-quoted raw string toggles. An odd number of `"""`
		// tokens on a line flips state.
		rawToggles := strings.Count(line, `"""`)
		priorRaw := inRawString
		if rawToggles%2 == 1 {
			inRawString = !inRawString
		}
		if priorRaw && inRawString {
			// Entirely inside a raw string — skip.
			continue
		}
		if inBlockComment {
			if strings.Contains(line, "*/") {
				inBlockComment = false
			}
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
				inBlockComment = true
			}
			continue
		}
		count++
	}
	return count
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

func (r *LargeClassRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *LargeClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if isTestFile(file.Path) {
		return nil
	}
	if strings.HasSuffix(file.Path, "Table.kt") || strings.HasSuffix(file.Path, "Tables.kt") ||
		strings.HasSuffix(file.Path, "Dao.kt") || strings.HasSuffix(file.Path, "DAO.kt") {
		return nil
	}
	lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx))
	if lines > r.AllowedLines {
		name := extractIdentifierFlat(file, idx)
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Class '%s' has %d lines (allowed: %d).", name, lines, r.AllowedLines))}
	}
	return nil
}

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

func (r *NestedBlockDepthRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *NestedBlockDepthRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	depth, line, exceeded := nestedBlockDepthExceedsFlat(file, idx, r.AllowedDepth)
	if exceeded {
		name := extractIdentifierFlat(file, idx)
		return []scanner.Finding{r.Finding(file, line, 1,
			fmt.Sprintf("Function '%s' has a nested block depth of %d (allowed: %d).", name, depth, r.AllowedDepth))}
	}
	return nil
}

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

func (r *CyclomaticComplexMethodRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *CyclomaticComplexMethodRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip override `equals(other: Any?)` and `hashCode()` methods —
	// these are mechanical boilerplate whose complexity scales with the
	// number of fields, not with author-created control flow.
	if file.FlatHasModifier(idx, "override") {
		name := extractIdentifierFlat(file, idx)
		if name == "equals" || name == "hashCode" {
			return nil
		}
	}
	// Skip pure-boolean predicate functions whose body is a single `return`
	// of a conjunction/disjunction chain (`return isX() || isY() || isZ()`).
	// These are flat classifier tables — semantically a single predicate,
	// not control flow.
	if isPureBooleanPredicateFlat(file, idx) {
		return nil
	}
	// Dedup: if the function is also going to trigger LongMethod
	// (>60 significant lines by default), suppress the complexity finding —
	// LongMethod already tells the author to refactor, and big methods
	// almost always carry elevated complexity. Reporting both is noise.
	if lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx)); lines > 60 {
		return nil
	}
	complexity, line, exceeded := cyclomaticComplexityExceedsFlat(file, idx, r.AllowedComplexity, r.IgnoreSimpleWhenEntries)
	if exceeded {
		name := extractIdentifierFlat(file, idx)
		return []scanner.Finding{r.Finding(file, line, 1,
			fmt.Sprintf("Function '%s' has a cyclomatic complexity of %d (allowed: %d).", name, complexity, r.AllowedComplexity))}
	}
	return nil
}

func isPureBooleanPredicateFlat(file *scanner.File, fn uint32) bool {
	body := file.FlatFindChild(fn, "function_body")
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
	stmts := file.FlatFindChild(body, "statements")
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
			stmts := file.FlatFindChild(child, "statements")
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

func (r *CognitiveComplexMethodRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *CognitiveComplexMethodRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	metrics := getComplexityMetricsFlat(idx, file)
	if metrics.cognitive > r.AllowedComplexity {
		name := extractIdentifierFlat(file, idx)
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Function '%s' has a cognitive complexity of %d (allowed: %d).", name, metrics.cognitive, r.AllowedComplexity))}
	}
	return nil
}

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

func (r *ComplexConditionRule) NodeTypes() []string {
	return []string{"if_expression", "while_statement"}
}

func isPureDisjunctionOrConjunctionFlat(file *scanner.File, root uint32) bool {
	hasConj := false
	hasDisj := false
	var walk func(uint32, bool)
	walk = func(idx uint32, inBody bool) {
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

func (r *ComplexConditionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	condOps := countLogicalOperatorsOutsideBodiesFlat(file, idx)
	if condOps > r.AllowedConditions {
		// Exempt pure-disjunction or pure-conjunction chains — these are
		// much easier to reason about than mixed && / || combinations.
		if isPureDisjunctionOrConjunctionFlat(file, idx) {
			return nil
		}
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Complex condition with %d logical operators (allowed: %d).", condOps, r.AllowedConditions))}
	}
	return nil
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

func (r *ComplexInterfaceRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *ComplexInterfaceRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check if this is an interface by looking for an "interface" keyword child
	if !file.FlatHasChildOfType(idx, "interface") {
		return nil
	}
	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	// Count only direct interface members; do not walk nested subtrees.
	members := countDirectClassMembersFlat(file, body)
	if members > r.AllowedDefinitions {
		name := extractIdentifierFlat(file, idx)
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Interface '%s' has %d members (allowed: %d).", name, members, r.AllowedDefinitions))}
	}
	return nil
}

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

func (r *LabeledExpressionRule) NodeTypes() []string { return []string{"label"} }

func (r *LabeledExpressionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Labeled expression '%s' detected. Consider refactoring to avoid labels.", strings.TrimSpace(file.FlatNodeText(idx))))}
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

func (r *LongParameterListRule) NodeTypes() []string {
	return []string{"function_declaration", "class_declaration"}
}

func (r *LongParameterListRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip test sources — test helpers legitimately take many parameters
	// to configure fixtures.
	if isTestFile(file.Path) {
		return nil
	}
	if file.FlatType(idx) == "function_declaration" {
		summary := getFunctionDeclSummaryFlat(file, idx)
		// Skip @Composable functions — Compose convention is many params
		// (state, multiple callbacks, modifier).
		if summary.hasComposable {
			return nil
		}
		// Skip override functions — the parameter shape is dictated by the
		// supertype, not the author. Override overrides.
		if summary.hasOverride {
			return nil
		}
		// Dedup: if the function is also going to trigger LongMethod
		// (>60 significant lines by default), suppress the parameter
		// list finding — LongMethod already tells the author to refactor.
		if lines := countSignificantLines(file, file.FlatRow(idx), flatEndRow(file, idx)); lines > 60 {
			return nil
		}
		if summary.paramsNode == 0 {
			return nil
		}
		params := 0
		limit := r.AllowedFunctionParameters
		for _, p := range summary.params {
			if r.IgnoreDefaultParameters && p.hasDefault {
				continue
			}
			if strings.Contains(file.FlatNodeText(p.idx), "->") {
				continue
			}
			params++
			if params > limit {
				name := summary.name
				return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Function '%s' has %d parameters (allowed: %d).", name, params, limit))}
			}
		}
	} else if file.FlatType(idx) == "class_declaration" {
		summary := getClassDeclSummaryFlat(file, idx)
		// Skip data classes if configured
		if r.IgnoreDataClasses && summary.hasData {
			return nil
		}
		if summary.hasParcelizeLike {
			return nil
		}
		// Skip pure value-holder classes — ones where every constructor
		// parameter is a `val`/`var` property. These are morally data
		// classes written as plain `class` for Java interop reasons
		// (@JvmField, abstract bases, custom equals, etc.).
		if len(summary.classParams) > 0 && r.IgnoreDataClasses {
			allProps := true
			for _, p := range summary.classParams {
				if !p.isProperty {
					allProps = false
					break
				}
			}
			if allProps {
				return nil
			}
		}
		clsName := summary.name
		if strings.HasSuffix(clsName, "ViewModel") || strings.HasSuffix(clsName, "Presenter") {
			return nil
		}
		params := 0
		limit := r.AllowedConstructorParameters
		for _, p := range summary.classParams {
			if r.IgnoreDefaultParameters && p.hasDefault {
				continue
			}
			if p.isFunctionType {
				continue
			}
			params++
			if params > limit {
				return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Constructor of '%s' has %d parameters (allowed: %d).", clsName, params, limit))}
			}
		}
	}
	return nil
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

func (r *MethodOverloadingRule) NodeTypes() []string { return []string{"source_file"} }

func (r *MethodOverloadingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	r.checkScopeFlat(idx, file, &findings)
	return findings
}

func (r *MethodOverloadingRule) checkScopeFlat(node uint32, file *scanner.File, findings *[]scanner.Finding) {
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
				r.checkScopeFlat(child, file, findings)
			}
		}
	case "class_declaration":
		body := file.FlatFindChild(node, "class_body")
		if body == 0 {
			return
		}
		forEachDirectClassBodyNodeFlat(file, body, func(child uint32) {
			switch file.FlatType(child) {
			case "function_declaration":
				addFunction(child)
			case "class_declaration":
				r.checkScopeFlat(child, file, findings)
			}
		})
	default:
		return
	}
	for name, count := range counts {
		if count > r.AllowedOverloads {
			*findings = append(*findings, r.Finding(file, firstLine[name], 1,
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

func (r *NamedArgumentsRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *NamedArgumentsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Try direct child first (simple O(1) lookup)
	args := file.FlatFindChild(idx, "value_arguments")
	if args == 0 {
		// Use pre-compiled query to find value_arguments inside call_suffix
		callSuffix := file.FlatFindChild(idx, "call_suffix")
		if callSuffix != 0 {
			args = file.FlatFindChild(callSuffix, "value_arguments")
		}
	}
	if args == 0 {
		return nil
	}
	unnamed := 0
	for i := 0; i < file.FlatChildCount(args); i++ {
		child := file.FlatChild(args, i)
		if file.FlatType(child) == "value_argument" {
			// A named argument has a "value_argument_label" or "=" child
			isNamed := false
			for j := 0; j < file.FlatChildCount(child); j++ {
				childPart := file.FlatChild(child, j)
				ct := file.FlatType(childPart)
				if ct == "value_argument_label" || ct == "simple_identifier" {
					// Check if there's an = sign after the identifier
					if j+1 < file.FlatChildCount(child) && file.FlatNodeText(file.FlatChild(child, j+1)) == "=" {
						isNamed = true
						break
					}
				}
			}
			if !isNamed {
				unnamed++
			}
		}
	}
	if unnamed > r.AllowedArguments {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Function call has %d unnamed arguments (allowed: %d). Use named arguments.", unnamed, r.AllowedArguments))}
	}
	return nil
}

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

func (r *NestedScopeFunctionsRule) NodeTypes() []string { return []string{"call_expression"} }

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

func (r *NestedScopeFunctionsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check if this call_expression is a scope function
	name := extractCallNameFlat(file, idx)
	if !scopeFunctions[name] {
		return nil
	}
	// Count how many ancestor call_expressions are also scope functions
	depth := 0
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "call_expression" {
			pName := extractCallNameFlat(file, parent)
			if scopeFunctions[pName] {
				depth++
			}
		}
	}
	if depth > r.AllowedDepth {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Nested scope function '%s' at depth %d (allowed: %d).", name, depth, r.AllowedDepth))}
	}
	return nil
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

func (r *ReplaceSafeCallChainWithRunRule) NodeTypes() []string {
	return []string{"navigation_expression"}
}

func (r *ReplaceSafeCallChainWithRunRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Only process the outermost navigation_expression in a chain.
	// Skip if the parent is a navigation_expression (nested chain).
	if parent, ok := file.FlatParent(idx); ok {
		if file.FlatType(parent) == "navigation_expression" {
			return nil
		}
		// Also skip if parent is a call_expression whose parent is a navigation_expression,
		// e.g. a?.b()?.c() — the call_expression wraps the inner navigation_expression.
		if file.FlatType(parent) == "call_expression" {
			if grandparent, ok := file.FlatParent(parent); ok && file.FlatType(grandparent) == "navigation_expression" {
				return nil
			}
		}
	}

	count := countSafeCallsInChainFlat(file, idx)
	if count >= 3 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("Chain of %d safe calls. Consider using '?.run { }' to simplify.", count))}
	}
	return nil
}

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

func (r *StringLiteralDuplicationRule) NodeTypes() []string { return []string{"source_file"} }

func (r *StringLiteralDuplicationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	counts := make(map[string]int)
	firstLine := make(map[string]int)

	file.FlatWalkNodes(idx, "string_literal", func(strNode uint32) {
		text := file.FlatNodeText(strNode)
		if len(text) <= 3 {
			return
		}
		counts[text]++
		if _, ok := firstLine[text]; !ok {
			firstLine[text] = file.FlatRow(strNode) + 1
		}
	})

	var findings []scanner.Finding
	for text, count := range counts {
		if count > r.AllowedDuplications {
			findings = append(findings, r.Finding(file, firstLine[text], 1,
				fmt.Sprintf("String literal %s appears %d times (allowed: %d). Consider extracting to a constant.", text, count, r.AllowedDuplications)))
		}
	}
	return findings
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

func (r *TooManyFunctionsRule) NodeTypes() []string { return []string{"source_file"} }

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
		if mods := file.FlatFindChild(fnNode, "modifiers"); mods != 0 && strings.Contains(file.FlatNodeText(mods), "Deprecated") {
			return false
		}
	}
	return true
}

func (r *TooManyFunctionsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	topLevelCount := 0
	var classDecls []uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "function_declaration":
			if r.shouldCountFunctionFlat(child, file) {
				topLevelCount++
			}
		case "class_declaration":
			classDecls = append(classDecls, child)
		}
	}

	if topLevelCount > r.AllowedFunctionsPerFile {
		findings = append(findings, r.Finding(file, 1, 1,
			fmt.Sprintf("File has %d top-level functions (allowed: %d).", topLevelCount, r.AllowedFunctionsPerFile)))
	}

	for _, cls := range classDecls {
		// Skip sealed classes — they act as closed algebraic type hierarchies
		// where the function count is set by the contract, not the author.
		if file.FlatHasModifier(cls, "sealed") {
			continue
		}
		// Skip abstract classes — many functions are contract stubs.
		if file.FlatHasModifier(cls, "abstract") {
			continue
		}
		// Skip interface declarations — the method count is a contract
		// design decision, not an author complexity issue.
		if file.FlatHasChildOfType(cls, "interface") {
			continue
		}
		// Skip Dagger @Component / @Subcomponent / @Module types — their
		// function count is determined by the DI graph, not complexity.
		if flatHasAnnotationNamed(file, cls, "Component") ||
			flatHasAnnotationNamed(file, cls, "Subcomponent") ||
			flatHasAnnotationNamed(file, cls, "Module") ||
			flatHasAnnotationNamed(file, cls, "DependencyGraph") ||
			flatHasAnnotationNamed(file, cls, "GraphExtension") ||
			flatHasAnnotationNamed(file, cls, "ContributesTo") ||
			flatHasAnnotationNamed(file, cls, "BindingContainer") {
			continue
		}
		// Skip data access layer classes — Tables, Daos, and Repositories
		// often expose one function per query and their shape is driven by
		// schema/domain, not author discretion. Likewise MVVM shells
		// (Fragment/ViewModel/Activity/Screen) where function count is
		// driven by UI event handlers.
		clsName := extractIdentifierFlat(file, cls)
		if strings.HasSuffix(clsName, "Table") || strings.HasSuffix(clsName, "Tables") ||
			strings.HasSuffix(clsName, "Dao") || strings.HasSuffix(clsName, "DAO") ||
			strings.HasSuffix(clsName, "Repository") || strings.HasSuffix(clsName, "Store") ||
			strings.HasSuffix(clsName, "Fragment") || strings.HasSuffix(clsName, "ViewModel") ||
			strings.HasSuffix(clsName, "Activity") || strings.HasSuffix(clsName, "Screen") ||
			strings.HasSuffix(clsName, "View") || strings.HasSuffix(clsName, "Adapter") ||
			strings.HasSuffix(clsName, "Presenter") || strings.HasSuffix(clsName, "Manager") {
			continue
		}
		count := r.countFunctionsInClassFlat(cls, file)
		// Determine scope-specific limit
		limit := r.AllowedFunctionsPerFile
		if r.AllowedFunctionsPerClass > 0 {
			limit = r.AllowedFunctionsPerClass
		}
		if r.AllowedFunctionsPerInterface > 0 && file.FlatHasChildOfType(cls, "interface") {
			limit = r.AllowedFunctionsPerInterface
		} else if r.AllowedFunctionsPerObject > 0 && file.FlatHasChildOfType(cls, "object") {
			limit = r.AllowedFunctionsPerObject
		} else if r.AllowedFunctionsPerEnum > 0 && file.FlatHasChildOfType(cls, "enum") {
			limit = r.AllowedFunctionsPerEnum
		}
		if count > limit {
			name := extractIdentifierFlat(file, cls)
			findings = append(findings, r.Finding(file, file.FlatRow(cls)+1, 1,
				fmt.Sprintf("Class '%s' has %d functions (allowed: %d).", name, count, limit)))
		}
	}
	return findings
}

func (r *TooManyFunctionsRule) countFunctionsInClassFlat(cls uint32, file *scanner.File) int {
	body := file.FlatFindChild(cls, "class_body")
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
