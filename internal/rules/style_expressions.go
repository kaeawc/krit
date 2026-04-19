package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"sync"

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
	body := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return result
	}
	container := file.FlatFindChild(body, "statements")
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

func isLoggingOrCheckCallWithContentFlat(n uint32, file *scanner.File) bool {
	if n == 0 || file.FlatType(n) != "call_expression" {
		return false
	}
	if file.FlatChildCount(n) == 0 {
		return false
	}
	t := file.FlatNodeText(file.FlatChild(n, 0))
	name := t
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSpace(name)
	switch name {
	case "v", "d", "i", "w", "e", "wtf",
		"println", "print",
		"require", "check", "checkNotNull", "requireNotNull", "error",
		"assert", "assertNotNull", "assertNull",
		"trace", "debug", "info", "warn", "fatal":
		return true
	}
	for _, prefix := range []string{"Log.", "Timber.", "Logger.", "log.", "SignalLogger."} {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

func collectWhenDispatchJumpsFlat(fn uint32, file *scanner.File) map[int]bool {
	result := make(map[int]bool)
	if fn == 0 || file == nil {
		return result
	}
	body := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return result
	}
	container := file.FlatFindChild(body, "statements")
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
// This is distinct from UnsafeCast which flags ALL bare `as` casts.
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
	mods := file.FlatFindChild(idx, "modifiers")
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

func (r *VarCouldBeValRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if isTestFile(file.Path) {
		return nil
	}
	// Find the "var" keyword child node directly in the AST.
	var varKeyword uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "var" || file.FlatNodeTextEquals(child, "var") {
			varKeyword = child
			break
		}
	}
	if varKeyword == 0 {
		return nil
	}

	// Skip override var — can't change to val without changing the interface/superclass.
	if file.FlatHasModifier(idx, "override") {
		return nil
	}

	// Skip lateinit var — these are framework-initialized (DI, test mocks,
	// view bindings, etc.) via reflection, and the rule can't see the
	// reassignments.
	if file.FlatHasModifier(idx, "lateinit") {
		return nil
	}

	// Skip delegated properties (var x by ...) — the delegate controls mutability.
	if file.FlatFindChild(idx, "property_delegate") != 0 {
		return nil
	}

	// Skip vars with framework annotations that indicate external initialization.
	// These include DI annotations, mocking, view binding, etc.
	if hasFrameworkAnnotationFlat(file, idx) {
		return nil
	}

	// Skip properties with custom setters — they need var even if the backing
	// field is never directly assigned from outside.
	// In tree-sitter Kotlin, the setter is a sibling after the property_declaration
	// inside the class_body, not a child of the property_declaration itself.
	if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "setter" {
		return nil
	}
	// Also handle getter followed by setter: property_declaration -> getter -> setter
	if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "getter" {
		if nextNext, ok := file.FlatNextSibling(nextSib); ok && file.FlatType(nextNext) == "setter" {
			return nil
		}
	}

	// Only flag local variables (inside function bodies) and private class/object
	// properties. Non-private class properties could be reassigned from other files.
	parent, ok := file.FlatParent(idx)
	if !ok {
		return nil
	}
	isLocal := file.FlatType(parent) == "statements"
	isClassLevel := file.FlatType(parent) == "class_body"
	if isClassLevel {
		// Only flag private properties at class level.
		if !file.FlatHasModifier(idx, "private") {
			return nil
		}
	} else if !isLocal {
		// Top-level property — only flag if private.
		if file.FlatType(parent) == "source_file" && !file.FlatHasModifier(idx, "private") {
			return nil
		}
	}

	varName := ""
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "variable_declaration" {
			varName = extractIdentifierFlat(file, child)
			break
		}
	}
	if varName == "" {
		return nil
	}

	// Check reassignments in the immediate parent scope first, then fall
	// back to a file-wide scan if not found. File-wide scan catches
	// reassignments that the parent-scope walk misses due to nested object
	// expressions, lambdas, parse errors, or deeply-nested callbacks.
	reassigned := r.reassignedNamesFlat(parent, file)[varName]
	if !reassigned {
		// Fallback: look for bare `varName =` / `varName +=` / `varName++` etc.
		// anywhere in the file. This is conservative (may miss shadowing
		// cases) but dramatically reduces false positives.
		if varCouldBeValFileWideReassigned(file, varName) {
			reassigned = true
		}
	}

	if !reassigned {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("'var %s' is never reassigned. Use 'val' instead.", varName))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(varKeyword)),
			EndByte:     int(file.FlatEndByte(varKeyword)),
			Replacement: "val",
		}
		return []scanner.Finding{f}
	}
	return nil
}

// varCouldBeValFileWideReassigned returns true if the file contains a
// textual reassignment of the given name — `name =`, `name +=`, `name++`,
// `++name`, or `this.name =`. Matches are precise enough to avoid most
// false positives while catching reassignments hidden behind parse errors
// or nested scopes.
func varCouldBeValFileWideReassigned(file *scanner.File, name string) bool {
	// Build regexes: `\bname\s*(=|\+=|-=|\*=|/=|%=|\|=|&=|\^=|<<=|>>=|\+\+|--)`
	// and `(\+\+|--)\s*\bname\b`. Keep it simple with substring scanning:
	escName := regexp.QuoteMeta(name)
	assignRe := regexp.MustCompile(`\b` + escName + `\s*(=[^=]|\+=|-=|\*=|/=|%=|\|=|&=|\^=|<<=|>>=|\+\+|--)`)
	prefixRe := regexp.MustCompile(`(\+\+|--)\s*` + escName + `\b`)
	thisRe := regexp.MustCompile(`\bthis\.` + escName + `\s*=[^=]`)
	for _, line := range file.Lines {
		// Skip the declaration line itself (which contains `var name =`)
		// by excluding lines matching `var name` pattern.
		if strings.Contains(line, "var "+name) || strings.Contains(line, "val "+name) {
			continue
		}
		if assignRe.MatchString(line) || prefixRe.MatchString(line) || thisRe.MatchString(line) {
			return true
		}
	}
	return false
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
			lhsText := strings.TrimSpace(file.FlatNodeText(lhs))
			if lhsText == "" {
				return
			}
			reassigned[lhsText] = true
			if strings.HasPrefix(lhsText, "this.") {
				reassigned[strings.TrimPrefix(lhsText, "this.")] = true
			}
		case "postfix_expression":
			childText := strings.TrimSpace(file.FlatNodeText(child))
			if strings.HasSuffix(childText, "++") || strings.HasSuffix(childText, "--") {
				reassigned[strings.TrimSuffix(strings.TrimSuffix(childText, "++"), "--")] = true
			}
		case "prefix_expression":
			childText := strings.TrimSpace(file.FlatNodeText(child))
			if strings.HasPrefix(childText, "++") || strings.HasPrefix(childText, "--") {
				reassigned[strings.TrimPrefix(strings.TrimPrefix(childText, "++"), "--")] = true
			}
		}
	})

	if cached, loaded := r.scopeReassignments.LoadOrStore(key, reassigned); loaded {
		return cached.(map[string]bool)
	}
	return reassigned
}

// MayBeConstantRule detects top-level vals that could be const.
type MayBeConstantRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *MayBeConstantRule) Confidence() float64 { return 0.75 }

func (r *MayBeConstantRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *MayBeConstantRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip .kts script files — top-level vals compile as script-class members,
	// which don't allow const modifier.
	if strings.HasSuffix(file.Path, ".kts") {
		return nil
	}
	// Only top-level val with primitive/string type and constant initializer
	if parent, ok := file.FlatParent(idx); !ok || (file.FlatType(parent) != "source_file" && file.FlatType(parent) != "companion_object") {
		return nil
	}
	text := file.FlatNodeText(idx)
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "val ") {
		return nil
	}
	// Check modifiers - skip if already const
	if file.FlatHasModifier(idx, "const") {
		return nil
	}
	// Must have an initializer
	if !strings.Contains(text, "=") {
		return nil
	}
	// Check if the initializer is a constant expression
	parts := strings.SplitN(text, "=", 2)
	if len(parts) != 2 {
		return nil
	}
	init := strings.TrimSpace(parts[1])
	if isConstant(init) {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Property may be declared as 'const val'.")
		// Add const modifier
		mods := file.FlatFindChild(idx, "modifiers")
		if mods != 0 {
			modsText := file.FlatNodeText(mods)
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(mods)),
				EndByte:     int(file.FlatEndByte(mods)),
				Replacement: modsText + " const",
			}
		} else {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatStartByte(idx)) + 3,
				Replacement: "const val",
			}
		}
		return []scanner.Finding{f}
	}
	return nil
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

func (r *ModifierOrderRule) NodeTypes() []string { return []string{"modifiers"} }

func (r *ModifierOrderRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var mods []string
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "annotation", "line_comment", "multiline_comment":
			continue
		}
		text := strings.TrimSpace(file.FlatNodeText(child))
		if text != "" {
			mods = append(mods, text)
		}
	}
	if len(mods) <= 1 {
		return nil
	}
	// Check if mods are in the expected order
	lastIdx := -1
	for _, m := range mods {
		orderIdx := modifierIndex(m)
		if orderIdx < 0 {
			continue
		}
		if orderIdx < lastIdx {
			f := r.Finding(file, file.FlatRow(idx)+1, 1,
				"Modifiers are not in the recommended order.")
			// Build sorted modifier string
			sorted := sortModifiers(mods)
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: strings.Join(sorted, " "),
			}
			return []scanner.Finding{f}
		}
		lastIdx = orderIdx
	}
	return nil
}

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

func (r *FunctionOnlyReturningConstantRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *FunctionOnlyReturningConstantRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip override/open/abstract functions — they exist per contract
	if file.FlatHasModifier(idx, "override") ||
		file.FlatHasModifier(idx, "open") ||
		file.FlatHasModifier(idx, "abstract") {
		return nil
	}
	// Skip Kotlin Multiplatform `actual` implementations — they fulfill
	// an `expect fun` contract and cannot be replaced with a `const val`
	// without breaking the multiplatform declaration.
	if file.FlatHasModifier(idx, "actual") {
		return nil
	}
	// Skip dependency-injection provider functions. `@Provides`,
	// `@Binds`, `@BindsInstance`, `@IntoSet`, `@IntoMap` functions
	// form the binding graph of Dagger/Hilt/Metro/etc. — they look
	// like "returns a constant" but they are DI bindings and cannot
	// be rewritten as `const val`.
	if HasIgnoredAnnotation(file.FlatNodeText(idx),
		[]string{"Provides", "Binds", "BindsInstance", "BindsOptionalOf",
			"IntoSet", "IntoMap", "ElementsIntoSet", "Multibinds",
			"ContributesBinding", "ContributesMultibinding",
			"ContributesTo", "ContributesSubcomponent"}) {
		return nil
	}
	// Skip functions with parameters — they can't be replaced with a const val
	params := file.FlatFindChild(idx, "function_value_parameters")
	if params != 0 {
		paramText := file.FlatNodeText(params)
		if len(strings.TrimSpace(strings.Trim(paramText, "()"))) > 0 {
			return nil
		}
	}
	// Skip extension functions — they take a receiver
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatType(file.FlatChild(idx, i)) == "receiver_type" {
			return nil
		}
	}
	// Skip functions defined inside an interface — they serve as default
	// implementations for subclasses/implementations to override.
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if t := file.FlatType(p); t == "class_declaration" || t == "object_declaration" {
			// Check if it's an interface
			for i := 0; i < file.FlatChildCount(p); i++ {
				c := file.FlatChild(p, i)
				if ct := file.FlatType(c); ct == "interface" || (ct == "class" && file.FlatNodeTextEquals(c, "interface")) {
					return nil
				}
			}
			break
		}
	}
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}
	bodyText := strings.TrimSpace(file.FlatNodeText(body))
	// Expression body: = "constant"
	if strings.HasPrefix(bodyText, "=") {
		expr := strings.TrimSpace(strings.TrimPrefix(bodyText, "="))
		if isConstant(expr) {
			name := extractIdentifierFlat(file, idx)
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Function '%s' only returns a constant. Consider replacing with a const val.", name))}
		}
	}
	// Block body with single return
	inner := strings.TrimPrefix(bodyText, "{")
	inner = strings.TrimSuffix(inner, "}")
	inner = strings.TrimSpace(inner)
	if strings.HasPrefix(inner, "return ") && !strings.Contains(inner, "\n") {
		expr := strings.TrimPrefix(inner, "return ")
		if isConstant(expr) {
			name := extractIdentifierFlat(file, idx)
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Function '%s' only returns a constant. Consider replacing with a const val.", name))}
		}
	}
	return nil
}

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

func (r *LoopWithTooManyJumpStatementsRule) NodeTypes() []string {
	return []string{"for_statement", "while_statement", "do_while_statement"}
}

func (r *LoopWithTooManyJumpStatementsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	jumpCount := 0
	// Count break/continue only for THIS loop: stop descending when we
	// enter a nested loop (jumps there target the inner loop) or a lambda
	// / nested function (those control flow constructs don't target this
	// loop). Without this, the rule sums jumps across nested loops and
	// reports them all on the outermost one.
	var walk func(n uint32, depth int)
	walk = func(n uint32, depth int) {
		if n == 0 {
			return
		}
		if depth > 0 {
			switch file.FlatType(n) {
			case "for_statement", "while_statement", "do_while_statement",
				"lambda_literal", "function_declaration", "anonymous_function":
				return
			}
		}
		if file.FlatType(n) == "jump_expression" {
			text := file.FlatNodeText(n)
			if strings.HasPrefix(text, "break") || strings.HasPrefix(text, "continue") {
				jumpCount++
			}
		}
		for i := 0; i < file.FlatChildCount(n); i++ {
			walk(file.FlatChild(n, i), depth+1)
		}
	}
	walk(idx, 0)
	if jumpCount > r.MaxJumps {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Loop has %d jump statements, max allowed is %d.", jumpCount, r.MaxJumps))}
	}
	return nil
}

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

func (r *ExplicitItLambdaParameterRule) NodeTypes() []string {
	return []string{"lambda_literal"}
}

func (r *ExplicitItLambdaParameterRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	paramsNode := file.FlatFindChild(idx, "lambda_parameters")
	if paramsNode == 0 {
		return nil
	}

	// Collect parameter declarations (skip commas and whitespace tokens).
	var paramNodes []uint32
	for i := 0; i < file.FlatChildCount(paramsNode); i++ {
		child := file.FlatChild(paramsNode, i)
		if t := file.FlatType(child); t == "variable_declaration" || t == "simple_identifier" {
			paramNodes = append(paramNodes, child)
		}
	}

	// Only applies to single-parameter lambdas.
	if len(paramNodes) != 1 {
		return nil
	}

	param := paramNodes[0]
	var name string
	hasType := false
	if file.FlatType(param) == "simple_identifier" {
		name = file.FlatNodeText(param)
	} else {
		// variable_declaration: may have simple_identifier + type children
		id := file.FlatFindChild(param, "simple_identifier")
		if id != 0 {
			name = file.FlatNodeText(id)
		}
		// Check for type annotation
		if file.FlatFindChild(param, "user_type") != 0 || file.FlatFindChild(param, "nullable_type") != 0 ||
			file.FlatFindChild(param, "function_type") != 0 {
			hasType = true
		}
	}

	if name != "it" {
		return nil
	}

	var msg string
	if hasType {
		msg = "`it` should not be used as name for a lambda parameter."
	} else {
		msg = "Explicit 'it' lambda parameter is redundant. Use implicit 'it'."
	}

	f := r.Finding(file, file.FlatRow(idx)+1, 1, msg)

	// Only auto-fix when there is no type annotation (safe to just remove the parameter).
	if !hasType {
		// Find the arrow token "->"; the fix removes from "{" through the arrow.
		arrowNode := findArrowInLambdaFlat(file, idx)
		if arrowNode != 0 {
			// Replace from the opening "{" through the end of "->" (plus trailing space) with "{ ".
			arrowEnd := int(file.FlatEndByte(arrowNode))
			// Skip a single trailing space after -> if present
			if arrowEnd < len(file.Content) && file.Content[arrowEnd] == ' ' {
				arrowEnd++
			}
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     arrowEnd,
				Replacement: "{ ",
			}
		}
	}

	return []scanner.Finding{f}
}

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

func (r *ExplicitItLambdaMultipleParametersRule) NodeTypes() []string {
	return []string{"lambda_literal"}
}

func (r *ExplicitItLambdaMultipleParametersRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	paramsNode := file.FlatFindChild(idx, "lambda_parameters")
	if paramsNode == 0 {
		return nil
	}

	// Collect parameter names.
	var names []string
	for i := 0; i < file.FlatChildCount(paramsNode); i++ {
		child := file.FlatChild(paramsNode, i)
		var name string
		switch file.FlatType(child) {
		case "simple_identifier":
			name = file.FlatNodeText(child)
		case "variable_declaration":
			id := file.FlatFindChild(child, "simple_identifier")
			if id != 0 {
				name = file.FlatNodeText(id)
			}
		default:
			continue
		}
		if name != "" {
			names = append(names, name)
		}
	}

	if len(names) <= 1 {
		return nil
	}

	for _, name := range names {
		if name == "it" {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				"`it` should not be used as name for a lambda parameter.")}
		}
	}
	return nil
}

func (r *ExplicitItLambdaMultipleParametersRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

func isInsideLambdaUnderFlat(child, stopAt uint32, file *scanner.File) bool {
	for p, ok := file.FlatParent(child); ok && p != stopAt; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "lambda_literal" {
			return true
		}
	}
	return false
}
