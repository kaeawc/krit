package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func flatTrailingLambdaParts(file *scanner.File, suffix uint32) (lambda, params, stmts uint32) {
	if file == nil || suffix == 0 {
		return 0, 0, 0
	}
	lambda = flatCallSuffixLambdaNode(file, suffix)
	if lambda == 0 {
		return 0, 0, 0
	}
	if file.FlatType(lambda) == "annotated_lambda" {
		if lit := file.FlatFindChild(lambda, "lambda_literal"); lit != 0 {
			lambda = lit
		}
	}
	params = file.FlatFindChild(lambda, "lambda_parameters")
	stmts = file.FlatFindChild(lambda, "statements")
	return lambda, params, stmts
}

func flatTrailingLambdaText(file *scanner.File, suffix uint32) string {
	lambda, _, _ := flatTrailingLambdaParts(file, suffix)
	if lambda == 0 {
		return ""
	}
	return file.FlatNodeText(lambda)
}

func flatReceiverCallExpression(file *scanner.File, idx uint32) uint32 {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return 0
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	if file.FlatType(receiver) == "call_expression" {
		return receiver
	}
	return 0
}

func flatApplyBodyReferencesThis(file *scanner.File, stmts uint32) bool {
	if file == nil || stmts == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(stmts, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "this_expression":
			found = true
		case "call_expression":
			callee := file.FlatChild(candidate, 0)
			if file.FlatType(callee) != "simple_identifier" {
				return
			}
			switch file.FlatNodeText(callee) {
			case "println", "print", "require", "check", "error", "TODO",
				"listOf", "mapOf", "setOf", "arrayOf",
				"emptyList", "emptyMap", "emptySet",
				"mutableListOf", "mutableMapOf", "mutableSetOf",
				"buildList", "buildMap", "buildSet",
				"lazy", "run", "with", "also", "let", "apply",
				"repeat", "synchronized", "maxOf", "minOf",
				"compareBy", "compareByDescending",
				"sortedBy", "sortedByDescending",
				"hashMapOf", "linkedMapOf", "hashSetOf", "linkedSetOf":
				return
			default:
				found = true
			}
		case "simple_identifier":
			parent, ok := file.FlatParent(candidate)
			if !ok {
				return
			}
			if file.FlatNodeTextEquals(candidate, "it") {
				return
			}
			if file.FlatType(parent) == "navigation_expression" {
				if file.FlatNamedChildCount(parent) > 0 && file.FlatNamedChild(parent, 0) == candidate {
					found = true
					return
				}
			}
			if file.FlatType(parent) == "statements" {
				found = true
				return
			}
			if file.FlatType(parent) == "assignment" || file.FlatType(parent) == "directly_assignable_expression" {
				if file.FlatChildCount(parent) > 0 && file.FlatChild(parent, 0) == candidate {
					found = true
					return
				}
			}
		}
	})
	return found
}

func flatFilterPredicateText(file *scanner.File, filterCall uint32) string {
	suffix := file.FlatFindChild(filterCall, "call_suffix")
	if suffix == 0 {
		return ""
	}
	return flatTrailingLambdaText(file, suffix)
}

func flatFilterCheckReceiver(filterCall uint32, file *scanner.File, resolver typeinfer.TypeResolver) bool {
	navExpr, _ := flatCallExpressionParts(file, filterCall)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	if receiver == 0 {
		return false
	}
	receiverName := file.FlatNodeText(receiver)
	if idx := strings.LastIndex(receiverName, "."); idx >= 0 {
		receiverName = receiverName[idx+1:]
	}
	resolved := resolver.ResolveByNameFlat(receiverName, receiver, file)
	if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
		if !filterReceiverTypes[resolved.Name] {
			return true
		}
	}
	return false
}

func flatAnyCallSuffixHasLambda(file *scanner.File, suffix uint32) bool {
	return flatCallSuffixLambdaNode(file, suffix) != 0
}

func flatAnyLambdaBodyText(file *scanner.File, suffix uint32) string {
	_, _, stmts := flatTrailingLambdaParts(file, suffix)
	if stmts == 0 || file.FlatChildCount(stmts) != 1 {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(file.FlatChild(stmts, 0)))
}

func flatAnyLambdaFullText(file *scanner.File, suffix uint32) string {
	lambda, _, _ := flatTrailingLambdaParts(file, suffix)
	if lambda == 0 {
		return ""
	}
	return file.FlatNodeText(lambda)
}

// RedundantHigherOrderMapUsageRule detects .map { it } (identity map).
type RedundantHigherOrderMapUsageRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unnecessary-call rule. Detection matches pattern shapes like
// .apply{}.let{}.also{} chains; type context determines whether the call
// is actually redundant. Classified per roadmap/17.
func (r *RedundantHigherOrderMapUsageRule) Confidence() float64 { return 0.75 }

func (r *RedundantHigherOrderMapUsageRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *RedundantHigherOrderMapUsageRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "map" {
		return nil
	}
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}
	suffix := file.FlatFindChild(idx, "call_suffix")
	if suffix == 0 {
		return nil
	}
	_, _, stmts := flatTrailingLambdaParts(file, suffix)
	if stmts == 0 {
		return nil
	}
	// Identity map: single statement that is just `it`.
	if file.FlatNamedChildCount(stmts) != 1 {
		return nil
	}
	stmt := file.FlatNamedChild(stmts, 0)
	if stmt == 0 || !file.FlatNodeTextEquals(stmt, "it") {
		return nil
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Redundant '.map { it }' — this is a no-op.")
	// Fix: remove the ".map { it }" portion from the call_expression.
	if child := flatLastChildOfType(file, navExpr, "navigation_suffix"); child != 0 {
		if file.FlatNodeText(file.FlatFindChild(child, "simple_identifier")) == "map" {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(child)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: "",
			}
		}
	}
	return []scanner.Finding{f}
}

// UnnecessaryApplyRule detects `.apply {}` where the lambda body never
// references the receiver (empty body, or body that only uses external
// references without any implicit/explicit `this` access).
type UnnecessaryApplyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unnecessary-call rule. Detection matches pattern shapes like
// .apply{}.let{}.also{} chains; type context determines whether the call
// is actually redundant. Classified per roadmap/17.
func (r *UnnecessaryApplyRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryApplyRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *UnnecessaryApplyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Determine if this call_expression invokes "apply".
	if flatCallExpressionName(file, idx) != "apply" {
		return nil
	}
	suffix := file.FlatFindChild(idx, "call_suffix")
	if suffix == 0 {
		return nil
	}
	// Verify that the .apply() call actually has a lambda block attached.
	// Without a lambda, this is NOT the Kotlin scope function — it's a
	// regular method named apply() (e.g., SharedPreferences.Editor.apply(),
	// KeyValueStore.Writer.apply(), RxJava Observable.apply()).
	if flatCallSuffixLambdaNode(file, suffix) == 0 {
		return nil
	}
	_, _, stmts := flatTrailingLambdaParts(file, suffix)
	if stmts == 0 {
		// Empty lambda body `{ }` — no statements node at all.
		return r.reportFlat(file, idx, "Unnecessary empty '.apply {}' block.")
	}
	if file.FlatChildCount(stmts) == 0 {
		// Statements node present but empty (whitespace only).
		return r.reportFlat(file, idx, "Unnecessary empty '.apply {}' block.")
	}

	// Non-empty body: flag if the body never references `this` (explicitly
	// or implicitly via unqualified member access / bare simple_identifier
	// calls that could resolve to the receiver).
	if !flatApplyBodyReferencesThis(file, stmts) {
		return r.reportFlat(file, idx,
			"'.apply {}' block does not reference the receiver — consider removing it or using 'also'/'let'.")
	}
	return nil
}

func (r *UnnecessaryApplyRule) reportFlat(file *scanner.File, idx uint32, msg string) []scanner.Finding {
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
	// Fix: remove the ".apply { ... }" portion from the call_expression.
	// The navigation_expression or receiver precedes it.
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr != 0 {
		if child := flatLastChildOfType(file, navExpr, "navigation_suffix"); child != 0 {
			if strings.Contains(file.FlatNodeText(child), "apply") {
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(child)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: "",
				}
			}
		}
	}
	return []scanner.Finding{f}
}

// UnnecessaryLetRule detects unnecessary .let {} calls via AST dispatch.
// Flags two patterns:
//  1. Identity let: `x?.let { it }` \u2014 the value can be used directly.
//  2. Single-call let: `x.let { it.foo() }` \u2014 can be replaced with `x.foo()`.
type UnnecessaryLetRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unnecessary-call rule. Detection matches pattern shapes like
// .apply{}.let{}.also{} chains; type context determines whether the call
// is actually redundant. Classified per roadmap/17.
func (r *UnnecessaryLetRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryLetRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *UnnecessaryLetRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// A .let{} call is: call_expression -> navigation_expression + call_suffix
	navNode, _ := flatCallExpressionParts(file, idx)
	if navNode == 0 {
		return nil
	}
	// The last child of navigation_expression is a navigation_suffix containing "let".
	navSuffix := flatLastChildOfType(file, navNode, "navigation_suffix")
	if navSuffix == 0 {
		return nil
	}
	// navigation_suffix children: operator ("." or "?.") + simple_identifier("let")
	funcIdent := file.FlatFindChild(navSuffix, "simple_identifier")
	if funcIdent == 0 || !file.FlatNodeTextEquals(funcIdent, "let") {
		return nil
	}

	isSafeCall := strings.Contains(file.FlatNodeText(navSuffix), "?.")

	// Get the lambda from the call_suffix using direct child traversal.
	suffix := file.FlatFindChild(idx, "call_suffix")
	if suffix == 0 {
		return nil
	}
	lambdaLit, params, stmts := flatTrailingLambdaParts(file, suffix)
	if lambdaLit == 0 {
		return nil
	}

	// Extract lambda parameter name (default "it") and body statements.
	paramName := "it"
	if params != 0 && file.FlatChildCount(params) > 0 {
		paramName = file.FlatNodeText(file.FlatChild(params, 0))
	}

	// Use stmts from the trailing lambda helper directly.
	if stmts == 0 || file.FlatChildCount(stmts) != 1 {
		return nil // multi-statement or empty lambda is not unnecessary
	}
	stmtText := strings.TrimSpace(file.FlatNodeText(file.FlatChild(stmts, 0)))

	// Pattern 1: Identity let \u2014 body is just the parameter (`it` or named param).
	if stmtText == paramName {
		msg := "Unnecessary '.let { " + paramName + " }' \u2014 the value can be used directly."
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(navSuffix)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: "",
		}
		return []scanner.Finding{f}
	}

	// Pattern 2: Single-call let \u2014 body is `it.something()` / `param.something()`.
	// Skip if the body contains a nested lambda (e.g. `it.filter { ... }`)
	// because rewriting could change scoping of `it` in the inner lambda.
	prefix := paramName + "."
	if strings.HasPrefix(stmtText, prefix) && !strings.Contains(stmtText, "{") {
		remainder := stmtText[len(paramName):] // includes leading "."
		if isSafeCall {
			msg := "Unnecessary '?.let { " + stmtText + " }' \u2014 can be replaced with '?." + stmtText[len(prefix):] + "'."
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(navSuffix)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: "?" + remainder,
			}
			return []scanner.Finding{f}
		}
		msg := "Unnecessary '.let { " + stmtText + " }' \u2014 can be replaced with '" + remainder + "'."
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(navSuffix)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: remainder,
		}
		return []scanner.Finding{f}
	}

	return nil
}

// UnnecessaryFilterRule detects .filter { pred }.first() patterns that can be
// simplified to .first { pred }. Covers first, firstOrNull, last, lastOrNull,
// single, singleOrNull, count, any, none, and property-access forms (size,
// isEmpty, isNotEmpty).
type UnnecessaryFilterRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *UnnecessaryFilterRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — pattern-matches
// filter().size/isEmpty()/count() chains without type confirmation of the
// receiver. Classified per roadmap/17.
func (r *UnnecessaryFilterRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryFilterRule) NodeTypes() []string { return []string{"call_expression"} }

// filterReceiverTypes are stdlib collection types for which filter+terminal can be simplified.
var filterReceiverTypes = map[string]bool{
	"List": true, "MutableList": true, "Collection": true,
	"Iterable": true, "Set": true, "MutableSet": true,
	"Sequence": true, "Map": true, "MutableMap": true,
}

// filterTerminatorCalls maps terminal method names (that take a predicate) to themselves.
var filterTerminatorCalls = map[string]string{
	"first": "first", "firstOrNull": "firstOrNull",
	"last": "last", "lastOrNull": "lastOrNull",
	"single": "single", "singleOrNull": "singleOrNull",
	"count": "count", "any": "any", "none": "none",
}

// filterTerminatorProps maps property-access terminals to their predicate equivalents.
var filterTerminatorProps = map[string]string{
	"size": "count", "isEmpty": "none", "isNotEmpty": "any",
}

func (r *UnnecessaryFilterRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}
	terminalName := flatNavigationExpressionLastIdentifier(file, navExpr)
	if terminalName == "" {
		return nil
	}

	replacement, isCall := filterTerminatorCalls[terminalName]
	if !isCall {
		return nil
	}

	if flatLastChildOfType(file, navExpr, "navigation_suffix") == 0 {
		return nil
	}

	// The terminal call_suffix must have empty arguments.
	callSuffix := file.FlatFindChild(idx, "call_suffix")
	if callSuffix == 0 {
		return nil
	}
	if flatCallSuffixHasArgs(file, callSuffix) {
		return nil
	}

	// The receiver of the navigation_expression must be a .filter { } call.
	filterCall := file.FlatNamedChild(navExpr, 0)
	if filterCall == 0 || file.FlatType(filterCall) != "call_expression" {
		return nil
	}
	if flatCallExpressionName(file, filterCall) != "filter" {
		return nil
	}
	predText := flatFilterPredicateText(file, filterCall)
	if predText == "" {
		return nil
	}

	// Optional: verify receiver type via resolver.
	if r.resolver != nil {
		if skip := flatFilterCheckReceiver(filterCall, file, r.resolver); skip {
			return nil
		}
	}

	msg := fmt.Sprintf("Replace '.filter %s.%s()' with '.%s %s'.",
		predText, terminalName, replacement, predText)
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)

	// Auto-fix: replace from the start of the .filter navigation_suffix through end of node.
	filterNavExpr, _ := flatCallExpressionParts(file, filterCall)
	filterNavSuffix := flatLastChildOfType(file, filterNavExpr, "navigation_suffix")
	if filterNavSuffix != 0 {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(filterNavSuffix)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: "." + replacement + " " + predText,
		}
	}

	return []scanner.Finding{f}
}

// UnnecessaryAnyRule detects unnecessary .any { true }, .any { it }, and
// .none { true } calls that can be replaced with .isNotEmpty() or .isEmpty().
// Also detects .filter { pred }.any() which should be .any { pred }.
type UnnecessaryAnyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unnecessary-call rule. Detection matches pattern shapes like
// .apply{}.let{}.also{} chains; type context determines whether the call
// is actually redundant. Classified per roadmap/17.
func (r *UnnecessaryAnyRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryAnyRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *UnnecessaryAnyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	methodName := flatCallExpressionName(file, idx)
	if methodName == "" {
		return nil
	}
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}

	switch methodName {
	case "any", "none":
		return r.checkAnyNoneLambda(idx, navExpr, methodName, file)
	default:
		return nil
	}
}

// checkAnyNoneLambda detects .any { true }, .any { it }, and .none { true }.
func (r *UnnecessaryAnyRule) checkAnyNoneLambda(idx uint32, navExpr uint32, methodName string, file *scanner.File) []scanner.Finding {
	suffix := file.FlatFindChild(idx, "call_suffix")
	if suffix == 0 {
		return nil
	}

	// Check for .filter { pred }.any() pattern: call_suffix with empty parens, no lambda.
	if methodName == "any" && !flatAnyCallSuffixHasLambda(file, suffix) {
		return r.checkFilterAny(idx, navExpr, suffix, file)
	}

	// Extract lambda body text for .any { true } / .any { it } / .none { true }.
	bodyText := flatAnyLambdaBodyText(file, suffix)
	if bodyText == "" {
		return nil
	}

	// Compute the byte offset where the navigation_suffix starts (the dot before method name).
	navSuffix := flatLastChildOfType(file, navExpr, "navigation_suffix")
	if navSuffix == 0 {
		return nil
	}
	dotStart := int(file.FlatStartByte(navSuffix))

	switch {
	case methodName == "any" && (bodyText == "true" || bodyText == "it"):
		// .any { true } or .any { it } -> .isNotEmpty()
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"Replace '.any { "+bodyText+" }' with '.isNotEmpty()'.")
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   dotStart,
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: ".isNotEmpty()",
		}
		return []scanner.Finding{f}

	case methodName == "none" && bodyText == "true":
		// .none { true } -> .isEmpty()
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"Replace '.none { true }' with '.isEmpty()'.")
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   dotStart,
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: ".isEmpty()",
		}
		return []scanner.Finding{f}
	}

	return nil
}

// checkFilterAny detects .filter { pred }.any() and suggests .any { pred }.
func (r *UnnecessaryAnyRule) checkFilterAny(idx uint32, navExpr uint32, callSuffix uint32, file *scanner.File) []scanner.Finding {
	// The call_suffix must have empty arguments (no lambda, no value args).
	if flatCallSuffixHasArgs(file, callSuffix) {
		return nil
	}

	// The receiver of the navigation_expression should be a .filter { } call.
	filterCall := file.FlatNamedChild(navExpr, 0)
	if filterCall == 0 || file.FlatType(filterCall) != "call_expression" {
		return nil
	}
	if flatCallExpressionName(file, filterCall) != "filter" {
		return nil
	}

	predText := flatAnyLambdaFullText(file, file.FlatFindChild(filterCall, "call_suffix"))
	if predText == "" {
		return nil
	}

	// Build the receiver text (everything before .filter).
	receiverNode := file.FlatNamedChild(file.FlatFindChild(filterCall, "navigation_expression"), 0)
	if receiverNode == 0 {
		return nil
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Use .any { predicate } instead of .filter { predicate }.any().")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatEndByte(receiverNode)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: ".any " + predText,
	}
	return []scanner.Finding{f}
}

// UnnecessaryBracesAroundTrailingLambdaRule detects unnecessary parentheses around trailing lambdas.
// Pattern: `foo() { lambda }` should be `foo { lambda }`.
type UnnecessaryBracesAroundTrailingLambdaRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unnecessary-call rule. Detection matches pattern shapes like
// .apply{}.let{}.also{} chains; type context determines whether the call
// is actually redundant. Classified per roadmap/17.
func (r *UnnecessaryBracesAroundTrailingLambdaRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryBracesAroundTrailingLambdaRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *UnnecessaryBracesAroundTrailingLambdaRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// We're looking for a call_expression whose structure is:
	//   call_expression
	//     call_expression          <-- inner: the actual call with empty ()
	//       navigation_expression | simple_identifier
	//       call_suffix
	//         value_arguments  "()"   <-- empty
	//     call_suffix
	//       annotated_lambda       <-- the trailing lambda
	//
	// The outer call_expression is what tree-sitter produces for `foo() { lambda }`.

	// Must have exactly 2 children: inner call_expression + call_suffix with lambda.
	if file.FlatChildCount(idx) < 2 {
		return nil
	}

	// First child must be a call_expression (the `foo()` part).
	innerCall := file.FlatChild(idx, 0)
	if innerCall == 0 || file.FlatType(innerCall) != "call_expression" {
		return nil
	}

	// Second child must be a call_suffix containing an annotated_lambda.
	outerSuffix := file.FlatChild(idx, 1)
	if outerSuffix == 0 || file.FlatType(outerSuffix) != "call_suffix" {
		return nil
	}
	if flatCallSuffixLambdaNode(file, outerSuffix) == 0 {
		return nil
	}

	// The inner call_expression must end with a call_suffix that has empty value_arguments.
	innerSuffix := file.FlatFindChild(innerCall, "call_suffix")
	if innerSuffix == 0 {
		return nil
	}
	args := file.FlatFindChild(innerSuffix, "value_arguments")
	if args == 0 {
		return nil
	}

	// Check that value_arguments contains no actual value_argument children (i.e., it's empty `()`).
	for i := 0; i < file.FlatChildCount(args); i++ {
		if file.FlatType(file.FlatChild(args, i)) == "value_argument" {
			return nil
		}
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(innerSuffix)+1,
		"Empty parentheses before trailing lambda can be removed.")

	// Fix: remove the empty "()" (the inner call_suffix).
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(innerSuffix)),
		EndByte:     int(file.FlatEndByte(innerSuffix)),
		Replacement: "",
	}

	return []scanner.Finding{f}
}

// UnnecessaryFullyQualifiedNameRule detects fully qualified names that have a matching import.
type UnnecessaryFullyQualifiedNameRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unnecessary-call rule. Detection matches pattern shapes like
// .apply{}.let{}.also{} chains; type context determines whether the call
// is actually redundant. Classified per roadmap/17.
func (r *UnnecessaryFullyQualifiedNameRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryFullyQualifiedNameRule) NodeTypes() []string {
	return []string{"navigation_expression"}
}

func (r *UnnecessaryFullyQualifiedNameRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	// Skip nodes that don't look like qualified names (must have at least one dot)
	if !strings.Contains(text, ".") {
		return nil
	}
	// Collect imports from file
	imports := make(map[string]bool)
	for _, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			imp := strings.TrimPrefix(trimmed, "import ")
			imp = strings.TrimSpace(imp)
			imports[imp] = true
		}
	}
	for imp := range imports {
		parts := strings.Split(imp, ".")
		if len(parts) < 2 {
			continue
		}
		if strings.Contains(text, imp) {
			shortName := parts[len(parts)-1]
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("Unnecessary fully qualified name '%s'. Use '%s' since it's already imported.", imp, shortName))}
		}
	}
	return nil
}

// UnnecessaryReversedRule detects chained sort+reverse (or reverse+sort) patterns
// that can be replaced by a single sort call.
// Examples:
//
//	.sorted().reversed()             -> .sortedDescending()
//	.sortedDescending().reversed()   -> .sorted()
//	.sortedBy{}.reversed()           -> .sortedByDescending{}
//	.sortedByDescending{}.reversed() -> .sortedBy{}
//	.reversed().sorted()             -> .sortedDescending()
//
// Also detects asReversed() in place of reversed().
type UnnecessaryReversedRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unnecessary-call rule. Detection matches pattern shapes like
// .apply{}.let{}.also{} chains; type context determines whether the call
// is actually redundant. Classified per roadmap/17.
func (r *UnnecessaryReversedRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryReversedRule) NodeTypes() []string { return []string{"call_expression"} }

// unnRevSortOpposites maps sort functions to their opposite.
var unnRevSortOpposites = map[string]string{
	"sorted":             "sortedDescending",
	"sortedDescending":   "sorted",
	"sortedBy":           "sortedByDescending",
	"sortedByDescending": "sortedBy",
}

// unnRevReverseFuncs are the reverse functions we detect.
var unnRevReverseFuncs = map[string]bool{
	"reversed":   true,
	"asReversed": true,
}

// unnRevSortFuncs are the sort functions we detect.
var unnRevSortFuncs = map[string]bool{
	"sorted": true, "sortedDescending": true,
	"sortedBy": true, "sortedByDescending": true,
}

func (r *UnnecessaryReversedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Extract the method name of this call_expression.
	outerMethod := flatCallExpressionName(file, idx)
	if outerMethod == "" {
		return nil
	}

	// Get the receiver call_expression (the thing before .method()).
	receiverCall := flatReceiverCallExpression(file, idx)
	if receiverCall == 0 {
		return nil
	}
	innerMethod := flatCallExpressionName(file, receiverCall)
	if innerMethod == "" {
		return nil
	}

	// Determine which is sort and which is reverse.
	var sortMethod string
	if unnRevReverseFuncs[outerMethod] && unnRevSortFuncs[innerMethod] {
		// .sorted().reversed() pattern
		sortMethod = innerMethod
	} else if unnRevSortFuncs[outerMethod] && unnRevReverseFuncs[innerMethod] {
		// .reversed().sorted() pattern
		sortMethod = outerMethod
	} else {
		return nil
	}

	replacement, ok := unnRevSortOpposites[sortMethod]
	if !ok {
		return nil
	}

	msg := fmt.Sprintf("Replace '%s().%s()' with '%s()' for a single sort operation.",
		innerMethod, outerMethod, replacement)

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)

	// Build the fix: replace the full chained expression with receiver.replacement().
	navExpr, _ := flatCallExpressionParts(file, receiverCall)
	if navExpr != 0 && file.FlatNamedChildCount(navExpr) > 0 {
		innerReceiver := file.FlatNamedChild(navExpr, 0)
		innerReceiverText := file.FlatNodeText(innerReceiver)

		if unnRevReverseFuncs[outerMethod] {
			// Inner is sort, outer is reverse - preserve inner's call_suffix (lambda or args).
			sortCallSuffix := file.FlatFindChild(receiverCall, "call_suffix")
			suffixText := "()"
			if sortCallSuffix != 0 {
				suffixText = file.FlatNodeText(sortCallSuffix)
			}
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: innerReceiverText + "." + replacement + suffixText,
			}
		} else {
			// Inner is reverse, outer is sort - preserve outer's call_suffix.
			outerSuffix := file.FlatFindChild(idx, "call_suffix")
			suffixText := "()"
			if outerSuffix != 0 {
				suffixText = file.FlatNodeText(outerSuffix)
			}
			reverseReceiver := file.FlatNamedChild(navExpr, 0)
			if reverseReceiver != 0 {
				reverseReceiverText := file.FlatNodeText(reverseReceiver)
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: reverseReceiverText + "." + replacement + suffixText,
				}
			}
		}
	}

	return []scanner.Finding{f}
}
