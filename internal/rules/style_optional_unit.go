package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	optionalUnitReturnTypeMessage = "Unit return type is optional and can be omitted."
	optionalUnitInferredMessage   = "The function %s defines a return type of Unit. This is unnecessary and can safely be removed."
	optionalUnitStatementMessage  = "A single Unit expression is unnecessary and can safely be removed."
)

func (r *OptionalUnitRule) check(ctx *api.Context) {
	if ctx.File.FlatType(ctx.Idx) == "statements" {
		r.checkUnitStatements(ctx)
		return
	}
	r.checkExplicitUnitType(ctx)
	r.checkInferredUnit(ctx)
	r.checkReturnUnit(ctx)
}

// checkExplicitUnitType reports `: Unit` on a function.
func (r *OptionalUnitRule) checkExplicitUnitType(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	typeNode := explicitReturnTypeNode(file, idx)
	if typeNode == 0 || file.FlatNodeText(typeNode) != "Unit" {
		return
	}
	f := r.Finding(file, file.FlatRow(typeNode)+1, file.FlatCol(typeNode)+1, optionalUnitReturnTypeMessage)
	// With an expression body the declared Unit hides the body's own type (`= coroutineScope { ... }`),
	// so only a block body is safe to rewrite.
	if body, _ := file.FlatFindChild(idx, "function_body"); body != 0 &&
		strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(body)), "{") {
		if colon := precedingColon(file, idx, typeNode); colon != 0 {
			f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(colon)), EndByte: int(file.FlatEndByte(typeNode))}
		}
	}
	ctx.Emit(f)
}

// explicitReturnTypeNode returns the type node after the `:` of a function declaration, or 0.
func explicitReturnTypeNode(file *scanner.File, fn uint32) uint32 {
	seenColon := false
	for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
		switch {
		case file.FlatType(c) == ":":
			seenColon = true
		case seenColon && file.FlatType(c) == "user_type":
			return c
		case seenColon && file.FlatIsNamed(c):
			return 0
		}
	}
	return 0
}

// precedingColon returns the `:` token that directly precedes typeNode among fn's children, or 0.
func precedingColon(file *scanner.File, fn, typeNode uint32) uint32 {
	colon := uint32(0)
	for c := file.FlatFirstChild(fn); c != 0 && c != typeNode; c = file.FlatNextSib(c) {
		if file.FlatType(c) == ":" {
			colon = c
		}
	}
	return colon
}

// checkInferredUnit reports `fun f() = Unit`, which detekt checks for functions that are not overrides.
func (r *OptionalUnitRule) checkInferredUnit(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if explicitReturnTypeNode(file, idx) != 0 || file.FlatHasModifier(idx, "override") {
		return
	}
	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 || file.FlatNamedChildCount(body) != 1 {
		return
	}
	expr := file.FlatNamedChild(body, 0)
	if file.FlatType(expr) != "simple_identifier" || file.FlatNodeText(expr) != "Unit" {
		return
	}
	ctx.EmitAt(file.FlatRow(expr)+1, file.FlatCol(expr)+1, strings.Replace(optionalUnitInferredMessage, "%s", extractIdentifierFlat(file, idx), 1))
}

// checkReturnUnit reports `return Unit`, a krit addition beyond detekt's rule.
func (r *OptionalUnitRule) checkReturnUnit(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return
	}
	file.FlatWalkNodes(body, "jump_expression", func(jump uint32) {
		if file.FlatChildCount(jump) < 2 {
			return
		}
		first := file.FlatChild(jump, 0)
		if file.FlatType(first) != "return" {
			return
		}
		second := file.FlatChild(jump, 1)
		if file.FlatType(second) == "simple_identifier" && file.FlatNodeTextEquals(second, "Unit") {
			f := r.Finding(file, file.FlatRow(jump)+1, file.FlatCol(jump)+1, "return Unit is redundant and can be replaced with return.")
			f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatEndByte(first)), EndByte: int(file.FlatEndByte(second))}
			ctx.Emit(f)
		}
	})
}

// checkUnitStatements reports a block with a bare `Unit` statement. A statement followed by another one is
// always redundant. The last statement is redundant only when nothing uses the block's value, which needs types
// for lambdas, so those are left alone.
func (r *OptionalUnitRule) checkUnitStatements(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	last := lastStatementNode(file, idx)
	found := false
	for c := file.FlatFirstChild(idx); c != 0 && !found; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "simple_identifier" && file.FlatNodeText(c) == "Unit" {
			found = c != last || blockValueUnused(file, idx)
		}
	}
	if !found {
		return
	}
	// A lambda's block starts at its first statement, other blocks at their brace.
	at := idx
	if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) != "lambda_literal" {
		if prev, ok := file.FlatPrevSibling(idx); ok && file.FlatType(prev) == "{" {
			at = prev
		} else if file.FlatType(parent) == "function_body" {
			at = parent
		}
	}
	ctx.EmitAt(file.FlatRow(at)+1, file.FlatCol(at)+1, optionalUnitStatementMessage)
}

// lastStatementNode returns the last statement of a statements node, ignoring comments.
func lastStatementNode(file *scanner.File, statements uint32) uint32 {
	last := uint32(0)
	for c := file.FlatFirstChild(statements); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) && !isFlatCommentNode(file, c) {
			last = c
		}
	}
	return last
}

// blockValueUnused reports whether the value of a statements block is discarded, so its last statement is not a result.
func blockValueUnused(file *scanner.File, statements uint32) bool {
	parent, ok := file.FlatParent(statements)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "function_body", "anonymous_initializer", "secondary_constructor", "finally_block":
		return true
	case "control_structure_body":
		return controlBodyValueUnused(file, parent)
	case "try_expression":
		return expressionIsStatement(file, parent)
	case "catch_block":
		try, ok := file.FlatParent(parent)
		return ok && expressionIsStatement(file, try)
	}
	return false
}

// controlBodyValueUnused reports whether the value of an if, when or loop body is discarded.
func controlBodyValueUnused(file *scanner.File, body uint32) bool {
	owner, ok := file.FlatParent(body)
	if !ok {
		return false
	}
	switch file.FlatType(owner) {
	case "for_statement", "while_statement", "do_while_statement":
		return true
	case "if_expression":
		return expressionIsStatement(file, owner)
	case "when_entry":
		when, ok := file.FlatParent(owner)
		return ok && expressionIsStatement(file, when)
	}
	return false
}

// expressionIsStatement reports whether an if, when or try expression is used as a statement.
func expressionIsStatement(file *scanner.File, expr uint32) bool {
	parent, ok := file.FlatParent(expr)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "statements":
		return expr != lastStatementNode(file, parent) || blockValueUnused(file, parent)
	case "control_structure_body":
		return controlBodyValueUnused(file, parent)
	}
	return false
}
