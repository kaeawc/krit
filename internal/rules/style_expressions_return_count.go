package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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
func (r *ReturnCountRule) Confidence() float64 { return api.ConfidenceMedium }

type jumpMetrics struct {
	returns int
	throws  int
}

var (
	returnPrefix = []byte("return")
	throwPrefix  = []byte("throw")
)

func isNestedCallableBodyFlat(t string) bool {
	return t == "function_declaration" || t == "lambda_literal" || t == "anonymous_function"
}

func getJumpMetricsFlat(idx uint32, file *scanner.File) jumpMetrics {
	if idx == 0 || file == nil {
		return jumpMetrics{}
	}
	return filefacts.NodeFact(fileFactsCache(), file, idx, slotJumpMetrics, func() jumpMetrics {
		var metrics jumpMetrics
		var walk func(uint32)
		walk = func(current uint32) {
			if current == 0 {
				return
			}
			if current != idx {
				if isNestedCallableBodyFlat(file.FlatType(current)) {
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
		return metrics
	})
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
// Entries with empty FQN lists are accepted on identifier shape alone and must
// be filtered separately.
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

// fileImportsFQN returns true when the file contains an import
// header whose (alias-stripped) path matches exactly `fqn` or
// `<package-of-fqn>.*`. Results are memoized via the shared filefacts
// cache (per-run, per-file).
func fileImportsFQN(file *scanner.File, fqn string) bool {
	if file == nil || fqn == "" {
		return false
	}
	return fileFactsCache().Imports(file).HasFQN(fqn)
}

func cleanImportHeaderTextWithAlias(text string) string {
	text = strings.TrimSpace(text)
	if nl := strings.IndexAny(text, "\r\n"); nl >= 0 {
		text = strings.TrimSpace(text[:nl])
	}
	text = strings.TrimPrefix(text, "import ")
	text = strings.TrimSuffix(text, ";")
	text = strings.TrimSpace(text)
	return text
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
	if isNestedCallableBodyFlat(file.FlatType(n)) {
		return
	}
	if file.FlatType(n) == "jump_expression" {
		out[int(file.FlatStartByte(n))] = true
	}
	for i := 0; i < file.FlatNamedChildCount(n); i++ {
		collectJumpStartsFlat(file.FlatNamedChild(n, i), out, file)
	}
}
