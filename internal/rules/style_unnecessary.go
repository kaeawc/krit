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
		if lit, ok := file.FlatFindChild(lambda, "lambda_literal"); ok {
			lambda = lit
		}
	}
	params, _ = file.FlatFindChild(lambda, "lambda_parameters")
	stmts, _ = file.FlatFindChild(lambda, "statements")
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

func flatFilterPredicateText(file *scanner.File, filterCall uint32) string {
	suffix, _ := file.FlatFindChild(filterCall, "call_suffix")
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

// UnnecessaryApplyRule detects `receiver.apply { stmt }` whose single statement only
// reaches the receiver (see style_unnecessary_apply.go).
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

// UnnecessaryFilterRule detects .filter { pred }.first() patterns that can be
// simplified to .first { pred }. Covers first, firstOrNull, last, lastOrNull,
// single, singleOrNull, count, any, none, and property-access forms (size,
// isEmpty, isNotEmpty).
type UnnecessaryFilterRule struct {
	FlatDispatchBase
	BaseRule
}

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
	// detekt also reports these: no element satisfies the predicate exactly when none does, and so on.
	"isEmpty": "none", "isNotEmpty": "any",
}

// UnnecessaryAnyRule detects unnecessary .any { true } and .none { true } calls
// that can be replaced with .any() or .none().
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

var knownListFactories = map[string]bool{
	"listOf": true, "mutableListOf": true, "arrayListOf": true, "emptyList": true, "listOfNotNull": true,
	"buildList": true, "List": true, "MutableList": true, "ArrayList": true,
}

var knownListOperations = map[string]bool{
	"toList": true, "toMutableList": true, "filter": true, "filterNot": true, "filterNotNull": true,
	"sorted": true, "sortedBy": true, "sortedDescending": true, "sortedWith": true, "reversed": true,
	"take": true, "drop": true, "distinct": true, "flatMap": true, "map": true, "mapNotNull": true,
}

// mapReceiverIsKnownList reports whether the receiver is provably a List from its factory call or declared type.
func mapReceiverIsKnownList(file *scanner.File, call, receiver uint32) bool {
	switch file.FlatType(receiver) {
	case "call_expression":
		name := flatCallNameAny(file, receiver)
		return knownListFactories[name] || knownListOperations[name]
	case "simple_identifier":
		name := file.FlatNodeText(receiver)
		scope, ok := flatEnclosingCallable(file, call)
		if !ok {
			scope = 0
		}
		known := false
		for _, nodeType := range []string{"property_declaration", "parameter"} {
			file.FlatWalkNodes(scope, nodeType, func(decl uint32) {
				if known || extractIdentifierFlat(file, decl) != name {
					return
				}
				if nodeType == "parameter" {
					if t, found := file.FlatFindChild(decl, "user_type"); found {
						known = declaredTypeIsList(file.FlatNodeText(t))
					}
					return
				}
				if t, found := file.FlatFindChild(decl, "user_type"); found {
					known = declaredTypeIsList(file.FlatNodeText(t))
					return
				}
				for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "call_expression" {
						known = knownListFactories[flatCallNameAny(file, child)] || knownListOperations[flatCallNameAny(file, child)]
					}
				}
			})
		}
		return known
	}
	return false
}

func declaredTypeIsList(typeText string) bool {
	return strings.HasPrefix(typeText, "List<") || strings.HasPrefix(typeText, "MutableList<") ||
		strings.HasPrefix(typeText, "ArrayList<")
}

// applyEnclosingScopeDeclares reports whether name is a parameter or a local declared before the block in a scope that encloses it,
// up to the enclosing function. Those win over members of the apply receiver.
func applyEnclosingScopeDeclares(file *scanner.File, block uint32, name string) bool {
	blockStart := file.FlatStartByte(block)
	declares := func(node uint32) bool {
		if id, ok := file.FlatFindChild(node, "simple_identifier"); ok {
			return file.FlatNodeTextEquals(id, name)
		}
		return false
	}
	for scope, ok := file.FlatParent(block); ok; scope, ok = file.FlatParent(scope) {
		for child := file.FlatFirstChild(scope); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "property_declaration":
				if file.FlatStartByte(child) < blockStart {
					if v, ok := file.FlatFindChild(child, "variable_declaration"); ok && declares(v) {
						return true
					}
				}
			case "function_value_parameters", "lambda_parameters":
				for p := file.FlatFirstChild(child); p != 0; p = file.FlatNextSib(p) {
					switch file.FlatType(p) {
					case "parameter", "variable_declaration":
						if declares(p) {
							return true
						}
					}
				}
			case "variable_declaration":
				// A for-loop variable.
				if file.FlatType(scope) == "for_statement" && declares(child) {
					return true
				}
			}
		}
		if t := file.FlatType(scope); t == "function_declaration" || t == "anonymous_function" {
			return false
		}
	}
	return false
}

// applyScopeDeclares reports whether name is declared as a parameter or local variable anywhere under scope.
func applyScopeDeclares(file *scanner.File, scope uint32, name string) bool {
	declared := false
	file.FlatWalkAllNodes(scope, func(n uint32) {
		if declared {
			return
		}
		switch file.FlatType(n) {
		case "variable_declaration", "parameter", "class_parameter":
			if id, ok := file.FlatFindChild(n, "simple_identifier"); ok && file.FlatNodeTextEquals(id, name) {
				declared = true
			}
		}
	})
	return declared
}
