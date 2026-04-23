package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// PropertyUsedBeforeDeclarationRule detects using property before declared.
// Uses DispatchBase on class_body to correctly identify class-level properties
// and avoid false positives from function bodies, lambdas, and init blocks.
// ---------------------------------------------------------------------------
type PropertyUsedBeforeDeclarationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *PropertyUsedBeforeDeclarationRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// UnconditionalJumpStatementInLoopRule detects loop with unconditional return/break.
// ---------------------------------------------------------------------------
type UnconditionalJumpStatementInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UnconditionalJumpStatementInLoopRule) Confidence() float64 { return 0.75 }


// ---------------------------------------------------------------------------
// UnnamedParameterUseRule detects function calls with many unnamed params.
// ---------------------------------------------------------------------------
type UnnamedParameterUseRule struct {
	FlatDispatchBase
	BaseRule
	AllowSingleParamUse bool
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UnnamedParameterUseRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// UnusedUnaryOperatorRule detects standalone +x or -x as expression statements
// whose result is never used. This catches the common bug where a line like
// `+ 3` appears to continue a previous expression but is actually parsed as
// a separate unary prefix expression with no effect.
// ---------------------------------------------------------------------------
type UnusedUnaryOperatorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UnusedUnaryOperatorRule) Confidence() float64 { return 0.75 }

// flatIsLastNamedChildOf checks if child is the last named child of parent.
func flatIsLastNamedChildOf(file *scanner.File, child, parent uint32) bool {
	count := file.FlatNamedChildCount(parent)
	if count == 0 {
		return false
	}
	return file.FlatNamedChild(parent, count-1) == child
}

// flatIsExpressionBlock checks if a "statements" node is part of a construct
// that uses its last expression as a value (if/when/lambda/try-catch bodies).
func flatIsExpressionBlock(file *scanner.File, stmts uint32) bool {
	parent, ok := file.FlatParent(stmts)
	if !ok {
		return false
	}
	// statements -> control_structure_body -> if_expression / when_entry
	// statements -> lambda_literal
	// statements -> catch_block -> try_expression
	// statements -> finally_block -> try_expression
	pt := file.FlatType(parent)
	if pt == "lambda_literal" {
		return true
	}
	if pt == "control_structure_body" {
		gp, ok := file.FlatParent(parent)
		if ok {
			gpt := file.FlatType(gp)
			if gpt == "if_expression" || gpt == "when_entry" || gpt == "when_expression" {
				return true
			}
		}
	}
	// try/catch/finally are expressions in Kotlin — last statement in the
	// try block, catch block, or finally block is the implicit return value.
	if pt == "catch_block" || pt == "finally_block" {
		gp, ok := file.FlatParent(parent)
		if ok && file.FlatType(gp) == "try_expression" {
			return true
		}
	}
	// statements directly inside try_expression (the try block body)
	if pt == "try_expression" {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// UselessPostfixExpressionRule detects `return x++` or `return x--`.
// ---------------------------------------------------------------------------
type UselessPostfixExpressionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence: tier-1 — the rule fires on a jump_expression whose `return`
// keyword is followed by a postfix_expression over a bare
// simple_identifier. That shape is unambiguous: the mutation happens
// after the value is produced, so the increment/decrement has no
// observable effect. Tree-sitter gives us this shape directly; no text
// heuristics are involved.
func (r *UselessPostfixExpressionRule) Confidence() float64 { return 0.95 }

// checkUselessPostfixFlat runs on jump_expression. Fires when the shape is
// `return <simple_identifier>++` or `return <simple_identifier>--`. The
// operand is intentionally required to be a simple_identifier (not a
// navigation_expression like `xs.size++`) because the fix — splitting into
// `x++` plus `return x` — is only safe when the incremented target is a
// single named variable.
func (r *UselessPostfixExpressionRule) checkUselessPostfixFlat(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	first := file.FlatFirstChild(idx)
	if first == 0 || file.FlatType(first) != "return" {
		return
	}
	operand := file.FlatNextSib(first)
	for operand != 0 && !file.FlatIsNamed(operand) {
		operand = file.FlatNextSib(operand)
	}
	if operand == 0 || file.FlatType(operand) != "postfix_expression" {
		return
	}
	target := file.FlatFirstChild(operand)
	for target != 0 && !file.FlatIsNamed(target) {
		target = file.FlatNextSib(target)
	}
	if target == 0 || file.FlatType(target) != "simple_identifier" {
		return
	}
	op := file.FlatNextSib(target)
	for op != 0 && file.FlatIsNamed(op) {
		op = file.FlatNextSib(op)
	}
	if op == 0 {
		return
	}
	opText := file.FlatType(op)
	if opText != "++" && opText != "--" {
		return
	}

	varName := file.FlatNodeText(target)
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Useless postfix expression in return statement. The increment/decrement has no effect.")
	// Replace the jump_expression's bytes (which start at `return`, so the
	// leading indent on the line is preserved). The second line is emitted
	// without indent — matching the rule's original behavior and its fix
	// fixture, which an external formatter is expected to re-indent.
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: varName + opText + "\n" + "return " + varName,
	}
	ctx.Emit(f)
}

func propertyDeclarationNameFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
	if varDecl == 0 {
		return ""
	}
	ident, _ := file.FlatFindChild(varDecl, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}
