package rules

import (
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



// UnnecessaryApplyRule detects `.apply {}` where the lambda body never
// references the receiver (empty body, or body that only uses external
// references without any implicit/explicit `this` access).
type UnnecessaryApplyRule struct {
	FlatDispatchBase
	BaseRule
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



// UnnecessaryAnyRule detects unnecessary .any { true }, .any { it }, and
// .none { true } calls that can be replaced with .isNotEmpty() or .isEmpty().
// Also detects .filter { pred }.any() which should be .any { pred }.
type UnnecessaryAnyRule struct {
	FlatDispatchBase
	BaseRule
}



// UnnecessaryBracesAroundTrailingLambdaRule detects unnecessary parentheses around trailing lambdas.
// Pattern: `foo() { lambda }` should be `foo { lambda }`.
type UnnecessaryBracesAroundTrailingLambdaRule struct {
	FlatDispatchBase
	BaseRule
}



// UnnecessaryFullyQualifiedNameRule detects fully qualified names that have a matching import.
type UnnecessaryFullyQualifiedNameRule struct {
	FlatDispatchBase
	BaseRule
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


