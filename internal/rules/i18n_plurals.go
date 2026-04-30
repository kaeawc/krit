package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// PluralsBuiltWithIfElseRule detects manual pluralization built with an
// if/else over `count == 1` whose branches produce string literals or
// templates instead of using getQuantityString / pluralStringResource.
type PluralsBuiltWithIfElseRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Heuristic shape
// match: identifier name + literal-1 comparison + string-producing
// branches. Classified per roadmap/17.
func (r *PluralsBuiltWithIfElseRule) Confidence() float64 { return 0.75 }

func (r *PluralsBuiltWithIfElseRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File

	cond, thenBody, elseBody := ifConditionThenElseBodiesFlat(file, idx)
	if cond == 0 || thenBody == 0 || elseBody == 0 {
		return
	}
	if !equalsCountVsOne(file, cond) {
		return
	}
	if !branchProducesString(file, thenBody) || !branchProducesString(file, elseBody) {
		return
	}

	ctx.EmitAt(file.FlatRow(idx)+1, 1,
		"Manual pluralization with if/else over count == 1. Use getQuantityString() or pluralStringResource() instead.")
}

// equalsCountVsOne reports whether the condition is an `==` between a
// count-like identifier and the integer literal 1.
func equalsCountVsOne(file *scanner.File, cond uint32) bool {
	if file.FlatType(cond) != "equality_expression" {
		return false
	}
	left, op, right := equalityOperands(file, cond)
	if op == 0 || file.FlatType(op) != "==" {
		return false
	}
	return identityVsOne(file, left, right) || identityVsOne(file, right, left)
}

func identityVsOne(file *scanner.File, idExpr, litExpr uint32) bool {
	if idExpr == 0 || litExpr == 0 {
		return false
	}
	if file.FlatType(idExpr) != "simple_identifier" {
		return false
	}
	if !pluralsCountNames[file.FlatNodeText(idExpr)] {
		return false
	}
	if file.FlatType(litExpr) != "integer_literal" {
		return false
	}
	switch file.FlatNodeText(litExpr) {
	case "1", "1L", "1l":
		return true
	}
	return false
}

// branchProducesString returns true when a control_structure_body's value
// expression is a Kotlin string literal or template. It walks past block
// wrappers and through `return <expr>` / `yield <expr>` style jumps.
func branchProducesString(file *scanner.File, body uint32) bool {
	expr := lastBranchExpression(file, body)
	if expr == 0 {
		return false
	}
	switch file.FlatType(expr) {
	case "line_string_literal", "string_literal", "multi_line_string_literal":
		return true
	case "jump_expression":
		// `return "..."` / similar — inspect the returned expression.
		for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
			if !file.FlatIsNamed(c) {
				continue
			}
			t := file.FlatType(c)
			if t == "line_string_literal" || t == "string_literal" || t == "multi_line_string_literal" {
				return true
			}
		}
	}
	return false
}

// lastBranchExpression returns the value-producing named child of a
// control_structure_body. For brace-wrapped bodies, it returns the last
// statement of the inner block.
func lastBranchExpression(file *scanner.File, body uint32) uint32 {
	last := flatLastNamedChild(file, body)
	if last != 0 && file.FlatType(last) == "statements" {
		return flatLastNamedChild(file, last)
	}
	return last
}
