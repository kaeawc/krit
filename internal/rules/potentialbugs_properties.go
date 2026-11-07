package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
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
func (r *PropertyUsedBeforeDeclarationRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// UnconditionalJumpStatementInLoopRule detects loop with unconditional return/break.
// ---------------------------------------------------------------------------
type UnconditionalJumpStatementInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UnconditionalJumpStatementInLoopRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *UnnamedParameterUseRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// UnusedUnaryOperatorRule detects a `+x` or `-x` that starts a line and whose
// result is never used. This catches the common bug where a line like `+ 3`
// appears to continue the expression above but is a separate statement.
// See checkUnusedUnaryOperator.
// ---------------------------------------------------------------------------
type UnusedUnaryOperatorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs properties rule. Detection is structural with heuristic
// fallbacks for flow-dependent cases. Classified per roadmap/17.
func (r *UnusedUnaryOperatorRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *UselessPostfixExpressionRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// checkUselessPostfixFlat runs on jump_expression. Fires when the shape is
// `return <simple_identifier>++` or `return <simple_identifier>--`. The
// operand is intentionally required to be a simple_identifier (not a
// navigation_expression like `xs.size++`) because the fix — splitting into
// `x++` plus `return x` — is only safe when the incremented target is a
// single named variable.
func (r *UselessPostfixExpressionRule) checkUselessPostfixFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) == "assignment" {
		r.checkAssignmentOfOwnPostfix(ctx)
		return
	}
	first := file.FlatFirstChild(idx)
	if first == 0 || file.FlatType(first) != "return" {
		return
	}
	operand := file.FlatNextSib(first)
	for operand != 0 && !file.FlatIsNamed(operand) {
		operand = file.FlatNextSib(operand)
	}
	if operand == 0 {
		return
	}
	if file.FlatType(operand) != "postfix_expression" {
		// detekt also checks postfix expressions that are direct children of the returned expression.
		holder := operand
		// PSI nests `a++.f()` as a qualified expression whose receiver is the postfix.
		if file.FlatType(operand) == "call_expression" {
			if nav := file.FlatFirstChild(operand); nav != 0 && file.FlatType(nav) == "navigation_expression" {
				holder = nav
			}
		}
		for child := file.FlatFirstChild(holder); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "postfix_expression" {
				continue
			}
			if _, ok := localPostfixTarget(file, idx, child); ok {
				ctx.Emit(r.Finding(file, file.FlatRow(child)+1, file.FlatCol(child)+1,
					"Useless postfix expression in return statement. The increment/decrement has no effect."))
			}
		}
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
	// A property keeps the new value after the return, so only a local variable loses it.
	if !returnTargetIsLocalVariable(file, idx, varName) {
		return
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Useless postfix expression in return statement. The increment/decrement has no effect.")
	// The local is gone after the return, so returning the old value is all that is left.
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: "return " + varName,
	}
	ctx.Emit(f)
}

// checkAssignmentOfOwnPostfix reports `x = x++`, which assigns the old value back to x.
func (r *UselessPostfixExpressionRule) checkAssignmentOfOwnPostfix(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	var left, right uint32
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "directly_assignable_expression":
			left = c
		case "postfix_expression":
			right = c
		case "=":
		default:
			if file.FlatIsNamed(c) {
				return
			}
		}
	}
	if left == 0 || right == 0 || file.FlatChildCount(idx) != 3 {
		return
	}
	operand := file.FlatFirstChild(right)
	for operand != 0 && !file.FlatIsNamed(operand) {
		operand = file.FlatNextSib(operand)
	}
	if operand == 0 || strings.Join(strings.Fields(file.FlatNodeText(operand)), "") != strings.Join(strings.Fields(file.FlatNodeText(left)), "") {
		return
	}
	if text := file.FlatNodeText(right); !strings.HasSuffix(text, "++") && !strings.HasSuffix(text, "--") {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Useless postfix expression: the assignment stores the value from before the increment/decrement back into the variable."))
}

// returnTargetIsLocalVariable reports a `var name` declared inside the function that holds node.
func returnTargetIsLocalVariable(file *scanner.File, node uint32, name string) bool {
	fn, ok := flatEnclosingAncestor(file, node, "function_declaration")
	if !ok {
		return false
	}
	found := false
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		if !found && propertyDeclarationNameFlat(file, decl) == name && file.FlatHasChildOfType(decl, "binding_pattern_kind") {
			found = true
		}
	})
	return found
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

// loopHasUnconditionalJump follows detekt: a break, continue or return that is a direct statement of the loop
// body, where a return only counts when no earlier statement holds a jump and it is not `x ?: break`.
func loopHasUnconditionalJump(file *scanner.File, loop uint32) bool {
	label := ""
	if prev, ok := file.FlatPrevSibling(loop); ok && file.FlatType(prev) == "label" {
		label = strings.TrimSuffix(file.FlatNodeText(prev), "@")
	}
	body, ok := file.FlatFindChild(loop, "control_structure_body")
	if !ok {
		return false
	}
	if file.FlatFirstChild(body) == 0 || file.FlatType(file.FlatFirstChild(body)) != "{" {
		for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				return loopStatementIsJump(file, c)
			}
		}
		return false
	}
	stmts, ok := file.FlatFindChild(body, "statements")
	if !ok {
		return false
	}
	for c := file.FlatFirstChild(stmts); c != 0; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) || isCommentNodeType(file.FlatType(c)) {
			continue
		}
		if label != "" {
			if cont := firstContinueIn(file, c); cont != 0 && jumpLabel(file, cont) == label {
				return false
			}
		}
		if loopStatementIsJump(file, c) {
			return true
		}
	}
	return false
}

func isCommentNodeType(t string) bool { return t == "line_comment" || t == "multiline_comment" }

// jumpKind returns "return", "break" or "continue" for a jump_expression, else "".
func jumpKind(file *scanner.File, n uint32) string {
	if file.FlatType(n) != "jump_expression" {
		return ""
	}
	first := file.FlatFirstChild(n)
	if first == 0 {
		return ""
	}
	return strings.TrimSuffix(file.FlatType(first), "@")
}

func jumpLabel(file *scanner.File, jump uint32) string {
	if l, ok := file.FlatFindChild(jump, "label"); ok {
		return strings.TrimSuffix(file.FlatNodeText(l), "@")
	}
	return ""
}

// firstContinueIn returns the first continue expression in document order, including n itself.
func firstContinueIn(file *scanner.File, n uint32) uint32 {
	if jumpKind(file, n) == "continue" {
		return n
	}
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if found := firstContinueIn(file, c); found != 0 {
			return found
		}
	}
	return 0
}

func loopStatementIsJump(file *scanner.File, n uint32) bool {
	switch jumpKind(file, n) {
	case "break", "continue":
		return true
	case "return":
		return !returnFollowedByElvisJump(file, n) && !jumpPrecedesSibling(file, n)
	}
	return false
}

// returnFollowedByElvisJump matches `return x ?: break` and `return x ?: continue`.
func returnFollowedByElvisJump(file *scanner.File, ret uint32) bool {
	for c := file.FlatFirstChild(ret); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "elvis_expression" {
			continue
		}
		last := uint32(0)
		for r := file.FlatFirstChild(c); r != 0; r = file.FlatNextSib(r) {
			if file.FlatIsNamed(r) {
				last = r
			}
		}
		k := jumpKind(file, last)
		return last != 0 && (k == "break" || k == "continue")
	}
	return false
}

// jumpPrecedesSibling reports whether an earlier sibling of n contains a jump statement anywhere inside.
func jumpPrecedesSibling(file *scanner.File, n uint32) bool {
	for prev, ok := file.FlatPrevSibling(n); ok; prev, ok = file.FlatPrevSibling(prev) {
		if subtreeHasLoopJump(file, prev) {
			return true
		}
	}
	return false
}

func subtreeHasLoopJump(file *scanner.File, n uint32) bool {
	if file.FlatType(n) == "jump_expression" && loopStatementIsJump(file, n) {
		return true
	}
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if subtreeHasLoopJump(file, c) {
			return true
		}
	}
	return false
}

// propertyRefCouldBeClassProperty filters identifiers that cannot name a property of the enclosing class: a
// member selected on another receiver (`other.id`), a named-argument label and a declared local name.
func propertyRefCouldBeClassProperty(file *scanner.File, ident uint32) bool {
	parent, ok := file.FlatParent(ident)
	if !ok {
		return true
	}
	switch file.FlatType(parent) {
	case "navigation_suffix":
		nav, ok := file.FlatParent(parent)
		if !ok {
			return false
		}
		recv := file.FlatFirstChild(nav)
		return recv != 0 && file.FlatType(recv) == "this_expression"
	case "variable_declaration":
		return false
	case "value_argument":
		next := file.FlatNextSib(ident)
		return next == 0 || file.FlatType(next) != "="
	}
	return true
}

// localDeclaredNames returns the names declared as locals under idx, which shadow class properties.
func localDeclaredNames(file *scanner.File, idx uint32) map[string]bool {
	names := map[string]bool{}
	file.FlatWalkNodes(idx, "variable_declaration", func(decl uint32) {
		if id, ok := file.FlatFindChild(decl, "simple_identifier"); ok {
			names[file.FlatNodeText(id)] = true
		}
	})
	return names
}

// startsAfterLineBreak reports whether only blanks separate the node from a preceding line break.
func startsAfterLineBreak(file *scanner.File, n uint32) bool {
	for i := int(file.FlatStartByte(n)) - 1; i >= 0; i-- {
		switch file.Content[i] {
		case ' ', '\t', '\r':
		case '\n':
			return true
		default:
			return false
		}
	}
	return false
}

// localPostfixTarget returns the name of the local variable that `name++` or `name--` changes.
func localPostfixTarget(file *scanner.File, ret, postfix uint32) (string, bool) {
	target := file.FlatFirstChild(postfix)
	if target == 0 || file.FlatType(target) != "simple_identifier" {
		return "", false
	}
	op := file.FlatNextSib(target)
	if op == 0 || (file.FlatType(op) != "++" && file.FlatType(op) != "--") {
		return "", false
	}
	name := file.FlatNodeText(target)
	return name, returnTargetIsLocalVariable(file, ret, name)
}
