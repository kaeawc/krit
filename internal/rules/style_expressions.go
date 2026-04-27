package rules

import (
	"bytes"
	"strings"
	"sync"

	rulesem "github.com/kaeawc/krit/internal/rules/semantics"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// ExpressionBodySyntaxRule detects single-expression functions that could use = syntax.
type ExpressionBodySyntaxRule struct {
	FlatDispatchBase
	BaseRule
	IncludeLineWrapping bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ExpressionBodySyntaxRule) Confidence() float64 { return 0.75 }

// ReturnCountRule limits the number of return statements in a function.
type ReturnCountRule struct {
	FlatDispatchBase
	BaseRule
	Max                     int
	ExcludedFunctions       []string // function names excluded from this rule
	ExcludeLabeled          bool
	ExcludeReturnFromLambda bool
	ExcludeGuardClauses     bool
}

// Confidence reports a tier-2 (medium) base confidence. The rule's
// counting is deterministic, but the threshold is a style preference
// with active disagreement (guard-clause handling, early-return
// patterns, when-expression with throw). The ExcludeGuardClauses and
// ExcludeReturnFromLambda knobs mitigate but don't eliminate the
// subjectivity. Medium keeps it out of strict default-confidence
// gates without removing it from the rule set.
func (r *ReturnCountRule) Confidence() float64 { return 0.75 }

type jumpMetrics struct {
	returns int
	throws  int
}

type jumpMetricsKey struct {
	filePath string
	start    int
	end      int
}

var jumpMetricsCache sync.Map
var (
	returnPrefix = []byte("return")
	throwPrefix  = []byte("throw")
)

func getJumpMetricsFlat(idx uint32, file *scanner.File) jumpMetrics {
	if idx == 0 || file == nil {
		return jumpMetrics{}
	}
	key := jumpMetricsKey{
		filePath: file.Path,
		start:    int(file.FlatStartByte(idx)),
		end:      int(file.FlatEndByte(idx)),
	}
	if cached, ok := jumpMetricsCache.Load(key); ok {
		return cached.(jumpMetrics)
	}
	var metrics jumpMetrics
	var walk func(uint32)
	walk = func(current uint32) {
		if current == 0 {
			return
		}
		if current != idx {
			t := file.FlatType(current)
			if t == "function_declaration" || t == "lambda_literal" {
				return
			}
		}
		if file.FlatType(current) == "jump_expression" {
			text := file.FlatNodeBytes(current)
			switch {
			case bytes.HasPrefix(text, returnPrefix):
				metrics.returns++
			case bytes.HasPrefix(text, throwPrefix):
				metrics.throws++
			}
		}
		for i := 0; i < file.FlatNamedChildCount(current); i++ {
			walk(file.FlatNamedChild(current, i))
		}
	}
	walk(idx)
	jumpMetricsCache.Store(key, metrics)
	return metrics
}

func collectGuardClauseJumpsFlat(fn uint32, file *scanner.File) map[int]bool {
	result := make(map[int]bool)
	if fn == 0 || file == nil {
		return result
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return result
	}
	container, _ := file.FlatFindChild(body, "statements")
	if container == 0 {
		container = body
	}
	for i := 0; i < file.FlatNamedChildCount(container); i++ {
		stmt := file.FlatNamedChild(container, i)
		if stmt == 0 {
			continue
		}
		if isGuardStatementFlat(stmt, file) {
			collectJumpStartsFlat(stmt, result, file)
			continue
		}
		if (file.FlatType(stmt) == "property_declaration" || file.FlatType(stmt) == "variable_declaration") &&
			!containsJumpExpressionFlat(stmt, file) {
			continue
		}
		if file.FlatType(stmt) == "assignment" && !containsJumpExpressionFlat(stmt, file) {
			continue
		}
		if file.FlatType(stmt) == "call_expression" && !containsJumpExpressionFlat(stmt, file) && isLoggingOrCheckCallWithContentFlat(stmt, file) {
			continue
		}
		break
	}
	return result
}

var bareLoggingOrCheckNames = map[string]struct{}{
	"println": {}, "print": {},
	"require": {}, "check": {}, "checkNotNull": {}, "requireNotNull": {}, "error": {},
	"assert": {}, "assertNotNull": {}, "assertNull": {},
	"trace": {}, "debug": {}, "info": {}, "warn": {}, "fatal": {},
}

var loggerReceiverMethodNames = map[string]struct{}{
	"v": {}, "d": {}, "i": {}, "w": {}, "e": {}, "wtf": {},
	"trace": {}, "debug": {}, "info": {}, "warn": {}, "error": {}, "fatal": {},
}

// knownLoggerReceiverFQNs maps the simple receiver name you'd see in
// source (`Log.d(...)`, `Timber.e(...)`) to the import FQN that must
// be present for the receiver to refer to a known logging API. An
// entry with an empty FQN (e.g. `SignalLogger`) means the receiver
// is a project-specific logger pattern that we accept on identifier
// shape alone — these must be filtered separately.
var knownLoggerReceiverFQNs = map[string][]string{
	"Log":    {"android.util.Log"},
	"Timber": {"timber.log.Timber"},
}

// isLoggingOrCheckCallWithContentFlat reports whether a
// `call_expression` node is a logging call or a precondition check
// (`require`, `check`, …). The classification is AST-only:
//   - Bare calls match a fixed allow-list of Kotlin stdlib / assertion
//     / framework top-level names.
//   - Qualified calls (`Log.d(...)`) require BOTH a `simple_identifier`
//     receiver matching a known logger name AND the corresponding
//     import FQN, or an import under one of the known logger packages
//     (slf4j, log4j, kotlin-logging, …). We never accept a bare
//     receiver prefix match against source text.
func isLoggingOrCheckCallWithContentFlat(n uint32, file *scanner.File) bool {
	if n == 0 || file == nil || file.FlatType(n) != "call_expression" {
		return false
	}
	callee := uint32(0)
	for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		callee = child
		break
	}
	if callee == 0 {
		return false
	}
	switch file.FlatType(callee) {
	case "simple_identifier":
		_, ok := bareLoggingOrCheckNames[file.FlatNodeText(callee)]
		return ok
	case "navigation_expression":
		method := flatNavigationExpressionLastIdentifier(file, callee)
		if _, ok := loggerReceiverMethodNames[method]; !ok {
			return false
		}
		recv := uint32(0)
		for child := file.FlatFirstChild(callee); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			recv = child
			break
		}
		if recv == 0 || file.FlatType(recv) != "simple_identifier" {
			return false
		}
		recvName := file.FlatNodeText(recv)
		if fqns, ok := knownLoggerReceiverFQNs[recvName]; ok {
			for _, fqn := range fqns {
				if fileImportsFQN(file, fqn) {
					return true
				}
			}
			return false
		}
		if _, aliases := buildLoggerImportsFromAST(file); aliases != nil {
			if _, ok := aliases[recvName]; ok {
				return true
			}
		}
		return false
	}
	return false
}

type importFQNCacheKey struct {
	path string
	fqn  string
}

var importFQNCache sync.Map

// fileImportsFQN returns true when the file contains an import
// header whose (alias-stripped) path matches exactly `fqn` or
// `<package-of-fqn>.*`. Results are cached per (file, fqn) pair.
func fileImportsFQN(file *scanner.File, fqn string) bool {
	if file == nil || fqn == "" {
		return false
	}
	key := importFQNCacheKey{path: file.Path, fqn: fqn}
	if cached, ok := importFQNCache.Load(key); ok {
		return cached.(bool)
	}
	wantWildcard := ""
	if idx := strings.LastIndex(fqn, "."); idx > 0 {
		wantWildcard = fqn[:idx] + ".*"
	}
	found := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		if found {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(node))
		text = strings.TrimPrefix(text, "import ")
		text = strings.TrimSuffix(text, ";")
		text = strings.TrimSpace(text)
		if alias := strings.Index(text, " as "); alias >= 0 {
			text = strings.TrimSpace(text[:alias])
		}
		if text == fqn || text == wantWildcard {
			found = true
		}
	})
	importFQNCache.Store(key, found)
	return found
}

func collectWhenDispatchJumpsFlat(fn uint32, file *scanner.File) map[int]bool {
	result := make(map[int]bool)
	if fn == 0 || file == nil {
		return result
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return result
	}
	container, _ := file.FlatFindChild(body, "statements")
	if container == 0 {
		container = body
	}
	var whenNode uint32
	var trailingJumps []uint32
	for i := 0; i < file.FlatNamedChildCount(container); i++ {
		stmt := file.FlatNamedChild(container, i)
		if stmt == 0 {
			continue
		}
		if whenNode == 0 {
			if (file.FlatType(stmt) == "property_declaration" || file.FlatType(stmt) == "variable_declaration") &&
				!containsJumpExpressionFlat(stmt, file) {
				continue
			}
			if file.FlatType(stmt) == "assignment" && !containsJumpExpressionFlat(stmt, file) {
				continue
			}
			if file.FlatType(stmt) == "when_expression" {
				whenNode = stmt
				continue
			}
			break
		}
		if file.FlatType(stmt) == "jump_expression" {
			trailingJumps = append(trailingJumps, stmt)
			continue
		}
		return result
	}
	if whenNode == 0 {
		return result
	}
	var branchReturns []uint32
	allBranchesReturn := true
	branchCount := 0
	for i := 0; i < file.FlatNamedChildCount(whenNode); i++ {
		entry := file.FlatNamedChild(whenNode, i)
		if entry == 0 || file.FlatType(entry) != "when_entry" {
			continue
		}
		branchCount++
		ret := findBranchReturnFlat(entry, file)
		if ret == 0 {
			allBranchesReturn = false
			break
		}
		branchReturns = append(branchReturns, ret)
	}
	if !allBranchesReturn || branchCount < 2 {
		return result
	}
	for _, r := range branchReturns {
		result[int(file.FlatStartByte(r))] = true
	}
	for _, t := range trailingJumps {
		result[int(file.FlatStartByte(t))] = true
	}
	return result
}

func findBranchReturnFlat(entry uint32, file *scanner.File) uint32 {
	if entry == 0 {
		return 0
	}
	var body uint32
	for i := 0; i < file.FlatNamedChildCount(entry); i++ {
		c := file.FlatNamedChild(entry, i)
		if c == 0 {
			continue
		}
		t := file.FlatType(c)
		if t == "when_condition" || t == "range_test" || t == "type_test" {
			continue
		}
		body = c
	}
	if body == 0 {
		return 0
	}
	return extractSoleReturnFlat(body, file)
}

func extractSoleReturnFlat(node uint32, file *scanner.File) uint32 {
	if node == 0 {
		return 0
	}
	switch file.FlatType(node) {
	case "jump_expression":
		return node
	case "control_structure_body", "statements":
		var only uint32
		for i := 0; i < file.FlatNamedChildCount(node); i++ {
			c := file.FlatNamedChild(node, i)
			if c == 0 {
				continue
			}
			if only != 0 {
				return 0
			}
			only = c
		}
		return extractSoleReturnFlat(only, file)
	}
	return 0
}

func containsJumpExpressionFlat(n uint32, file *scanner.File) bool {
	if n == 0 {
		return false
	}
	if file.FlatType(n) == "jump_expression" {
		return true
	}
	for i := 0; i < file.FlatNamedChildCount(n); i++ {
		if containsJumpExpressionFlat(file.FlatNamedChild(n, i), file) {
			return true
		}
	}
	return false
}

func isInsideWhenInitializerGuardFlat(jump, fn uint32, file *scanner.File) bool {
	if jump == 0 || fn == 0 {
		return false
	}
	var sawWhen bool
	for cur, ok := file.FlatParent(jump); ok && cur != fn; cur, ok = file.FlatParent(cur) {
		t := file.FlatType(cur)
		if t == "when_expression" {
			sawWhen = true
		}
		if t == "property_declaration" || t == "variable_declaration" {
			if !sawWhen {
				return false
			}
			for p, ok := file.FlatParent(cur); ok && p != fn; p, ok = file.FlatParent(p) {
				pt := file.FlatType(p)
				if pt == "if_expression" || pt == "when_expression" ||
					pt == "for_statement" || pt == "while_statement" ||
					pt == "do_while_statement" || pt == "control_structure_body" {
					return false
				}
				if pt == "statements" || pt == "function_body" {
					return true
				}
			}
			return true
		}
		if t == "function_declaration" || t == "lambda_literal" {
			return false
		}
	}
	return false
}

func isInsideInitializerGuardFlat(jump, fn uint32, file *scanner.File) bool {
	if jump == 0 || fn == 0 {
		return false
	}
	sawGuardContext := false
	for cur, ok := file.FlatParent(jump); ok && cur != fn; cur, ok = file.FlatParent(cur) {
		t := file.FlatType(cur)
		if t == "elvis_expression" || t == "catch_block" || t == "try_expression" {
			sawGuardContext = true
		}
		if t == "property_declaration" || t == "variable_declaration" {
			if !sawGuardContext {
				return false
			}
			for p, ok := file.FlatParent(cur); ok && p != fn; p, ok = file.FlatParent(p) {
				pt := file.FlatType(p)
				if pt == "if_expression" || pt == "when_expression" ||
					pt == "for_statement" || pt == "while_statement" ||
					pt == "do_while_statement" || pt == "control_structure_body" {
					return false
				}
				if pt == "statements" || pt == "function_body" {
					return true
				}
			}
			return true
		}
		if t == "function_declaration" || t == "lambda_literal" {
			return false
		}
	}
	return false
}

func isGuardStatementFlat(n uint32, file *scanner.File) bool {
	if n == 0 {
		return false
	}
	switch file.FlatType(n) {
	case "jump_expression":
		return true
	case "if_expression":
		for i := 0; i < file.FlatChildCount(n); i++ {
			if file.FlatType(file.FlatChild(n, i)) == "else" {
				return false
			}
		}
		return true
	}
	return false
}

func collectJumpStartsFlat(n uint32, out map[int]bool, file *scanner.File) {
	if n == 0 {
		return
	}
	if t := file.FlatType(n); t == "function_declaration" || t == "lambda_literal" {
		return
	}
	if file.FlatType(n) == "jump_expression" {
		out[int(file.FlatStartByte(n))] = true
	}
	for i := 0; i < file.FlatNamedChildCount(n); i++ {
		collectJumpStartsFlat(file.FlatNamedChild(n, i), out, file)
	}
}

// ThrowsCountRule limits the number of throw statements in a function.
type ThrowsCountRule struct {
	FlatDispatchBase
	BaseRule
	Max                 int
	ExcludeGuardClauses bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ThrowsCountRule) Confidence() float64 { return 0.75 }

func countJumpExpressionsFlat(root uint32, file *scanner.File, prefix string, limit int, accept func(uint32, string) bool) int {
	count := 0
	prefixBytes := returnPrefix
	if prefix == "throw" {
		prefixBytes = throwPrefix
	}
	var walk func(uint32) bool
	walk = func(node uint32) bool {
		if node == 0 {
			return false
		}
		if node != root && file.FlatType(node) == "function_declaration" {
			return false
		}
		if file.FlatType(node) == "jump_expression" {
			textBytes := file.FlatNodeBytes(node)
			if bytes.HasPrefix(textBytes, prefixBytes) {
				if accept == nil {
					count++
					if count > limit {
						return true
					}
				} else {
					text := string(textBytes)
					if accept(node, text) {
						count++
						if count > limit {
							return true
						}
					}
				}
			}
		}
		for i := 0; i < file.FlatNamedChildCount(node); i++ {
			if walk(file.FlatNamedChild(node, i)) {
				return true
			}
		}
		return false
	}
	walk(root)
	return count
}

// CollapsibleIfStatementsRule detects nested if without else.
type CollapsibleIfStatementsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *CollapsibleIfStatementsRule) Confidence() float64 { return 0.75 }

// SafeCastRule detects `if (x is Type) { x as Type }` patterns that should use `x as? Type`.
// This is distinct from UnsafeCast, which is reserved for casts that can never succeed.
// SafeCast only fires when an is-check + cast pattern is detected (the cast is redundant
// because the is-check already proves the type).
type SafeCastRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The rule
// matches the `if (x is T) { x as T }` pattern on text, which is
// narrow enough to avoid most false positives but can stumble on
// multi-branch or shadowed variables. Roadmap/17 notes that this
// rule and UnsafeCast fire on overlapping locations; medium
// confidence is appropriate for the redundant-cast half of that
// pair.
func (r *SafeCastRule) Confidence() float64 { return 0.75 }

// frameworkAnnotationNames identifies annotations indicating external
// initialization by a framework — these vars can't be analyzed for mutability.
var frameworkAnnotationNames = map[string]bool{
	"Mock":               true, // Mockito
	"MockK":              true,
	"RelaxedMockK":       true,
	"SpyK":               true,
	"InjectMocks":        true,
	"Inject":             true, // Dagger/JSR-330
	"Autowired":          true, // Spring
	"BindView":           true, // ButterKnife
	"BindViews":          true,
	"Captor":             true, // Mockito
	"Spy":                true,
	"Value":              true, // Spring
	"EJB":                true,
	"Resource":           true,
	"PersistenceContext": true,
}

func hasFrameworkAnnotationFlat(file *scanner.File, idx uint32) bool {
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	for i := 0; i < file.FlatChildCount(mods); i++ {
		child := file.FlatChild(mods, i)
		if file.FlatType(child) != "annotation" {
			continue
		}
		text := file.FlatNodeText(child)
		raw := text
		text = strings.TrimPrefix(text, "@")
		if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
			text = text[colonIdx+1:]
		}
		if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
			text = text[:parenIdx]
		}
		text = strings.TrimSpace(text)
		if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
			text = text[dotIdx+1:]
		}
		if frameworkAnnotationNames[text] {
			return true
		}
		if text == "Suppress" || text == "SuppressWarnings" {
			if strings.Contains(raw, `"unused"`) ||
				strings.Contains(raw, `"UNUSED_PARAMETER"`) ||
				strings.Contains(raw, `"UNUSED_VARIABLE"`) ||
				strings.Contains(raw, `"UnusedPrivateProperty"`) ||
				strings.Contains(raw, `"UnusedPrivateMember"`) ||
				strings.Contains(raw, `"UnusedPrivateFunction"`) ||
				strings.Contains(raw, `"UnusedVariable"`) {
				return true
			}
		}
	}
	return false
}

// VarCouldBeValRule detects var properties that are never reassigned.
type VarCouldBeValRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreLateinitVar  bool // if true, skip lateinit var declarations
	scopeReassignments sync.Map
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule uses per-file scope-reassignment tracking instead of full
// control-flow analysis. It correctly handles the common case (var
// declared and never reassigned in its enclosing scope) but misses
// nuances that flow analysis would catch — e.g. vars reassigned only
// on one branch of a when, or through a captured lambda. Medium
// confidence reflects the known analysis gap.
func (r *VarCouldBeValRule) Confidence() float64 { return 0.75 }

type scopeReassignmentsKey struct {
	filePath string
	start    int
	end      int
}

func (r *VarCouldBeValRule) reassignedNamesFlat(scope uint32, file *scanner.File) map[string]bool {
	key := scopeReassignmentsKey{
		filePath: file.Path,
		start:    int(file.FlatStartByte(scope)),
		end:      int(file.FlatEndByte(scope)),
	}
	if cached, ok := r.scopeReassignments.Load(key); ok {
		return cached.(map[string]bool)
	}

	reassigned := make(map[string]bool)
	file.FlatWalkAllNodes(scope, func(child uint32) {
		switch file.FlatType(child) {
		case "assignment", "augmented_assignment":
			if file.FlatChildCount(child) == 0 {
				return
			}
			lhs := file.FlatChild(child, 0)
			if name, ok := reassignedLocalOrThisNameFlat(file, lhs); ok {
				reassigned[name] = true
			}
		case "postfix_expression":
			if name, ok := reassignedPostfixNameFlat(file, child); ok {
				reassigned[name] = true
			}
		case "prefix_expression":
			if name, ok := reassignedPrefixNameFlat(file, child); ok {
				reassigned[name] = true
			}
		}
	})

	if cached, loaded := r.scopeReassignments.LoadOrStore(key, reassigned); loaded {
		return cached.(map[string]bool)
	}
	return reassigned
}

func reassignedLocalOrThisNameFlat(file *scanner.File, lhs uint32) (string, bool) {
	lhs = flatUnwrapParenExpr(file, lhs)
	switch file.FlatType(lhs) {
	case "simple_identifier":
		return file.FlatNodeText(lhs), true
	case "directly_assignable_expression":
		if file.FlatNamedChildCount(lhs) == 1 {
			return reassignedLocalOrThisNameFlat(file, file.FlatNamedChild(lhs, 0))
		}
	case "navigation_expression":
		return reassignedThisReceiverNameFlat(file, lhs)
	}
	return "", false
}

func reassignedThisReceiverNameFlat(file *scanner.File, nav uint32) (string, bool) {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChildCount(nav) == 0 {
		return "", false
	}
	first := flatUnwrapParenExpr(file, file.FlatNamedChild(nav, 0))
	if file.FlatType(first) != "this_expression" && !file.FlatNodeTextEquals(first, "this") {
		return "", false
	}
	name := flatNavigationExpressionLastIdentifier(file, nav)
	return name, name != ""
}

func reassignedPostfixNameFlat(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "postfix_expression" {
		return "", false
	}
	var operand uint32
	hasMutation := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "++") || file.FlatNodeTextEquals(child, "--") {
			hasMutation = true
			continue
		}
		if operand == 0 && file.FlatIsNamed(child) {
			operand = child
		}
	}
	if !hasMutation {
		return "", false
	}
	return reassignedLocalOrThisNameFlat(file, operand)
}

func reassignedPrefixNameFlat(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "prefix_expression" {
		return "", false
	}
	hasMutation := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "++") || file.FlatNodeTextEquals(child, "--") {
			hasMutation = true
			continue
		}
		if hasMutation && file.FlatIsNamed(child) {
			return reassignedLocalOrThisNameFlat(file, child)
		}
	}
	return "", false
}

// MayBeConstantRule detects top-level vals that could be const.
type MayBeConstantRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Initializers are
// checked structurally and same-file constant references are resolved when
// they share the same owner, but the finding remains a style preference.
func (r *MayBeConstantRule) Confidence() float64 { return 0.75 }

func mayBeConstantExpressionFlat(ctx *v2.Context, expr uint32) bool {
	if ctx == nil || ctx.File == nil || expr == 0 {
		return false
	}
	file := ctx.File
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		return !flatContainsStringInterpolation(file, expr)
	case "integer_literal", "long_literal", "real_literal", "hex_literal", "bin_literal", "character_literal", "boolean_literal":
		return true
	case "prefix_expression":
		return prefixConstantExpressionFlat(ctx, expr)
	case "simple_identifier", "navigation_expression":
		_, ok := rulesem.EvalConst(ctx, expr)
		return ok
	case "additive_expression", "multiplicative_expression":
		return binaryConstantExpressionFlat(ctx, expr)
	}
	return false
}

func prefixConstantExpressionFlat(ctx *v2.Context, expr uint32) bool {
	file := ctx.File
	seenSign := false
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "+") || file.FlatNodeTextEquals(child, "-") {
			seenSign = true
			continue
		}
		if seenSign && file.FlatIsNamed(child) {
			child = flatUnwrapParenExpr(file, child)
			switch file.FlatType(child) {
			case "integer_literal", "long_literal", "real_literal", "hex_literal", "bin_literal":
				return true
			default:
				return mayBeConstantExpressionFlat(ctx, child)
			}
		}
	}
	return false
}

func binaryConstantExpressionFlat(ctx *v2.Context, expr uint32) bool {
	file := ctx.File
	named := make([]uint32, 0, 2)
	validOp := false
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			named = append(named, child)
			continue
		}
		switch file.FlatNodeText(child) {
		case "+", "-", "*", "/", "%":
			validOp = true
		}
	}
	if !validOp || len(named) != 2 {
		return false
	}
	return mayBeConstantExpressionFlat(ctx, named[0]) && mayBeConstantExpressionFlat(ctx, named[1])
}

// ModifierOrderRule detects modifiers not in the recommended order.
type ModifierOrderRule struct {
	FlatDispatchBase
	BaseRule
}

var modifierOrder = []string{
	"public", "protected", "private", "internal",
	"expect", "actual",
	"final", "open", "abstract", "sealed",
	"const",
	"external",
	"override",
	"lateinit",
	"tailrec",
	"vararg",
	"suspend",
	"inner",
	"enum", "annotation", "fun",
	"companion",
	"inline", "noinline", "crossinline",
	"value",
	"infix",
	"operator",
	"data",
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ModifierOrderRule) Confidence() float64 { return 0.75 }

func modifierIndex(mod string) int {
	for i, m := range modifierOrder {
		if m == mod {
			return i
		}
	}
	return -1
}

func sortModifiers(mods []string) []string {
	result := make([]string, len(mods))
	copy(result, mods)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			ii := modifierIndex(result[i])
			jj := modifierIndex(result[j])
			if ii < 0 {
				ii = 9999
			}
			if jj < 0 {
				jj = 9999
			}
			if jj < ii {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// FunctionOnlyReturningConstantRule detects functions that only return a constant.
type FunctionOnlyReturningConstantRule struct {
	FlatDispatchBase
	BaseRule
	ExcludedFunctions         []string
	IgnoreOverridableFunction bool
	IgnoreActualFunction      bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *FunctionOnlyReturningConstantRule) Confidence() float64 { return 0.75 }

func isConstant(s string) bool {
	s = strings.TrimSpace(s)
	// String literal (but not string templates with interpolation)
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		if strings.Contains(s, "$") {
			return false
		}
		// Reject anything like "foo" + "bar" — only a single literal.
		inner := s[1 : len(s)-1]
		if strings.Contains(inner, "\"") {
			return false
		}
		return true
	}
	// Boolean literal
	if s == "true" || s == "false" {
		return true
	}
	// null literal
	if s == "null" {
		return true
	}
	// Pure numeric literal — must contain ONLY digits, decimal point, and
	// optional type suffix (L, F, f, .0, etc.). Reject anything with
	// operators, parentheses, identifiers, or whitespace.
	if len(s) == 0 {
		return false
	}
	// Reject if any character suggests an operator or compound expression.
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '(' || c == ')' ||
			c == '+' || c == '-' || c == '*' || c == '/' || c == '%' ||
			c == '&' || c == '|' || c == '^' || c == '!' || c == '~' ||
			c == '<' || c == '>' || c == '=' || c == '.' && false /* dot allowed for decimals */ {
			return false
		}
	}
	// Must start with a digit (possibly with a sign consumed above — but we
	// rejected signs, so a leading digit is required).
	if s[0] < '0' || s[0] > '9' {
		return false
	}
	// Accept if the remaining chars are digits, a single '.', or a type suffix.
	sawDot := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '.' {
			if sawDot {
				return false
			}
			sawDot = true
			continue
		}
		// Type suffix: L, F, f, d, _ (underscore separator)
		if c == 'L' || c == 'F' || c == 'f' || c == 'D' || c == 'd' || c == '_' || c == 'u' || c == 'U' {
			continue
		}
		return false
	}
	return true
}

// LoopWithTooManyJumpStatementsRule limits break/continue in loops.
type LoopWithTooManyJumpStatementsRule struct {
	FlatDispatchBase
	BaseRule
	MaxJumps int
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *LoopWithTooManyJumpStatementsRule) Confidence() float64 { return 0.75 }

// ExplicitItLambdaParameterRule detects `{ it -> ... }` using AST-based analysis.
// It finds lambda_literal nodes with exactly one parameter named "it" and flags them.
// When the parameter has a type annotation (e.g. `{ it: Int -> ... }`), a different
// message is used because the parameter cannot simply be removed.
type ExplicitItLambdaParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ExplicitItLambdaParameterRule) Confidence() float64 { return 0.75 }

func findArrowInLambdaFlat(file *scanner.File, lambda uint32) uint32 {
	for i := 0; i < file.FlatChildCount(lambda); i++ {
		child := file.FlatChild(lambda, i)
		if file.FlatNodeTextEquals(child, "->") {
			return child
		}
	}
	return 0
}

// ExplicitItLambdaMultipleParametersRule detects 'it' as a parameter name in
// lambdas with multiple parameters. Naming a parameter 'it' is confusing when
// there are other parameters because 'it' normally refers to the implicit
// single parameter.
type ExplicitItLambdaMultipleParametersRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ExplicitItLambdaMultipleParametersRule) Confidence() float64 { return 0.75 }

func isInsideLambdaUnderFlat(child, stopAt uint32, file *scanner.File) bool {
	for p, ok := file.FlatParent(child); ok && p != stopAt; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "lambda_literal" {
			return true
		}
	}
	return false
}
