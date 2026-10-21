package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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
func (r *NestedBlockDepthRule) Confidence() float64 { return api.ConfidenceMedium }

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

// functionBodyIsSingleWhenExpressionFlat reports whether the function's
// body consists of exactly one `when` expression — either as an
// expression body (`= when (...) { ... }`) or as a block whose only
// statement is a `when`. Used to honor
// CyclomaticComplexMethodRule.IgnoreSingleWhenExpression so that
// switch-style dispatch functions don't pop the threshold purely by
// virtue of having many when entries.
func functionBodyIsSingleWhenExpressionFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	body, ok := file.FlatFindChild(idx, "function_body")
	if !ok || body == 0 {
		return false
	}
	var sole uint32
	for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) {
			continue
		}
		if sole != 0 {
			return false
		}
		sole = c
	}
	if sole == 0 {
		return false
	}
	switch file.FlatType(sole) {
	case "when_expression":
		return true
	case "statements":
		var inner uint32
		for c := file.FlatFirstChild(sole); c != 0; c = file.FlatNextSib(c) {
			if !file.FlatIsNamed(c) {
				continue
			}
			if inner != 0 {
				return false
			}
			inner = c
		}
		return inner != 0 && file.FlatType(inner) == "when_expression"
	}
	return false
}

// functionIsLocalFlat reports whether the given function_declaration is
// nested inside another function/lambda body — i.e. it's a local
// function rather than a top-level or member declaration. Used to honor
// CyclomaticComplexMethodRule.IgnoreLocalFunctions and any future rules
// that need to distinguish member methods from local helpers.
func functionIsLocalFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration", "anonymous_function", "lambda_literal":
			return true
		case "class_body", "source_file":
			return false
		}
	}
	return false
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

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *CyclomaticComplexMethodRule) Confidence() float64 { return api.ConfidenceMedium }

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
func cyclomaticComplexityExceedsFlat(
	file *scanner.File,
	root uint32,
	allowed int,
	ignoreSimpleWhenEntries bool,
	ignoreNestingFunctions bool,
	nestingFunctions []string,
) (complexity int, line int, exceeded bool) {
	if file == nil || file.FlatTree == nil {
		return 0, 0, false
	}
	// The cached metrics walker stops at every lambda_literal — i.e. it
	// behaves like ignoreNestingFunctions=true with all lambdas treated
	// as nesting. That is only correct when IgnoreNestingFunctions is true
	// and the callee is in NestingFunctions. For any other configuration we
	// re-walk to count the decisions inside non-nesting lambdas as well.
	useFreshWalker := !ignoreNestingFunctions || len(nestingFunctions) == 0
	if useFreshWalker {
		complexity = countCyclomaticAcrossLambdasFlat(file, root, ignoreSimpleWhenEntries, false, nil)
	} else {
		complexity = countCyclomaticAcrossLambdasFlat(file, root, ignoreSimpleWhenEntries, true, nestingFunctions)
	}
	if complexity > allowed {
		line = file.FlatRow(root) + 1
		exceeded = true
	}
	return complexity, line, exceeded
}

// countCyclomaticAcrossLambdasFlat is the cyclomatic counter that
// honors ignoreNestingFunctions / nestingFunctions semantics. Unlike
// the cached walker in collectComplexityMetricsFlat, this walker may
// recurse INTO lambda bodies — so decisions inside a `forEach { ... }`
// (or any other lambda whose callee isn't on the skip list) contribute
// to the count.
//
// Walk rules:
//   - Always stop at nested function_declaration (those are separate
//     functions and analyzed independently).
//   - At a lambda_literal: when ignoreNestingFunctions is true AND the
//     enclosing call_expression's callee name is in nestingFunctions,
//     stop (skip the lambda body). Otherwise, recurse into the lambda.
//   - Decision-type nodes increment the counter, with the same
//     `ignoreSimpleWhenEntries` filter the cached walker applies.
//   - elvis_expression contributes one decision, consistent with the
//     cached walker.
func countCyclomaticAcrossLambdasFlat(
	file *scanner.File,
	root uint32,
	ignoreSimpleWhenEntries bool,
	ignoreNestingFunctions bool,
	nestingFunctions []string,
) int {
	skip := make(map[string]bool, len(nestingFunctions))
	if ignoreNestingFunctions {
		for _, n := range nestingFunctions {
			n = strings.TrimSpace(n)
			if n != "" {
				skip[n] = true
			}
		}
	}
	cyclomatic := 1
	var walk func(uint32)
	walk = func(idx uint32) {
		nodeType := file.FlatType(idx)
		if idx != root && nodeType == "function_declaration" {
			return
		}
		if idx != root && nodeType == "lambda_literal" {
			if ignoreNestingFunctions {
				if name := lambdaCalleeNameForNestingFlat(file, idx); name != "" && skip[name] {
					return
				}
			}
			// fall through and recurse into the lambda
		}
		if decisionTypes[nodeType] {
			counts := nodeType != "when_entry" || !ignoreSimpleWhenEntries || !isSimpleWhenEntryFlat(file, idx)
			if counts {
				cyclomatic++
			}
		}
		if nodeType == "elvis_expression" {
			cyclomatic++
		}
		t := file.FlatTree
		for c := t.FirstChildren[idx]; c != 0; c = t.NextSibs[c] {
			if !t.NodeIsNamed(c) {
				continue
			}
			walk(c)
		}
	}
	walk(root)
	return cyclomatic
}

// lambdaCalleeNameForNestingFlat returns the simple name of the call
// that "owns" the given lambda_literal node — i.e. for a trailing
// lambda like `list.forEach { ... }`, it returns "forEach". The walk
// looks at the lambda's enclosing call_expression (skipping any
// annotated_lambda / value_arguments wrappers) and pulls the callee
// name from there. Returns "" when the lambda isn't attached to a
// callable node.
func lambdaCalleeNameForNestingFlat(file *scanner.File, lambda uint32) string {
	parent, ok := file.FlatParent(lambda)
	if !ok {
		return ""
	}
	// Skip wrappers tree-sitter places between the lambda and its call.
	// For trailing lambdas the chain is
	//     lambda_literal → annotated_lambda → call_suffix → call_expression
	// for argument-list lambdas it's
	//     lambda_literal → lambda_argument? → value_argument → value_arguments → call_expression
	for {
		switch file.FlatType(parent) {
		case "annotated_lambda", "call_suffix", "lambda_argument", "value_arguments", "value_argument":
			p, ok := file.FlatParent(parent)
			if !ok {
				return ""
			}
			parent = p
			continue
		}
		break
	}
	if file.FlatType(parent) != "call_expression" {
		return ""
	}
	return extractCallNameFlat(file, parent)
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
func (r *CognitiveComplexMethodRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComplexConditionRule) Confidence() float64 { return api.ConfidenceMedium }

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
	return !hasConj || !hasDisj
}
