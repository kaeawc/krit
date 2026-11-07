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

// CyclomaticComplexMethodRule counts decision points per function.
type CyclomaticComplexMethodRule struct {
	FlatDispatchBase
	BaseRule
	AllowedComplexity          int
	IgnoreSimpleWhenEntries    bool
	IgnoreSingleWhenExpression bool
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
// statement is a `when` or `return when`, as in detekt. Used to honor
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
		return inner != 0 && (file.FlatType(inner) == "when_expression" || isReturnOfWhenFlat(file, inner))
	case "jump_expression":
		return isReturnOfWhenFlat(file, sole)
	}
	return false
}

// isReturnOfWhenFlat reports `return when (...) { ... }`, which detekt treats like a bare `when` body.
func isReturnOfWhenFlat(file *scanner.File, node uint32) bool {
	if file.FlatType(node) != "jump_expression" || !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(node)), "return") {
		return false
	}
	named := uint32(0)
	count := 0
	for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			named = c
			count++
		}
	}
	return count == 1 && file.FlatType(named) == "when_expression"
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *CyclomaticComplexMethodRule) Confidence() float64 { return api.ConfidenceMedium }

// lostWhenEntriesFlat counts `->` arrows left in ERROR nodes inside one `when`, not looking into lambdas,
// functions or nested `when` expressions.
func lostWhenEntriesFlat(file *scanner.File, when uint32) int {
	lost := 0
	var walk func(uint32)
	walk = func(idx uint32) {
		for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "lambda_literal", "function_declaration", "when_expression":
				continue
			case "ERROR":
				if strings.TrimSpace(file.FlatNodeText(c)) == "->" || file.FlatHasChildOfType(c, "->") {
					lost++
				}
			}
			walk(c)
		}
	}
	walk(when)
	return lost
}

// CognitiveComplexMethodRule measures cognitive complexity, weighting nesting depth.
type CognitiveComplexMethodRule struct {
	FlatDispatchBase
	BaseRule
	AllowedComplexity int
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

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *ComplexConditionRule) Confidence() float64 { return api.ConfidenceMedium }

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
