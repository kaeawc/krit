package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// PrecompileUnreachableCodeRule flags statements that follow an
// unconditional jump (return/throw/break/continue) in the same
// statements block. Mirrors kotlinc's UNREACHABLE_CODE.
//
// Conservative by construction: fires only when the jump is in
// statement position (direct child of a `statements` node), so
// expression-position jumps like `x ?: return` and `if (b) return else
// ...` are skipped. Looking only at the jump's direct parent keeps the
// rule local — nested lambdas/anonymous functions cannot escape its
// scope.
type PrecompileUnreachableCodeRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrecompileUnreachableCodeRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *PrecompileUnreachableCodeRule) check(ctx *api.Context) {
	// The compiler also knows calls that return Nothing, `while (true)` and exhaustive branches, so its verdict
	// replaces the syntactic scan whenever it analyzed the file.
	if compilerAnalyzedFile(ctx) {
		resolver := ctx.Resolver.(*oracle.CompositeResolver)
		for _, d := range oracleLookupDiagnosticsForFlatRange(resolver.Oracle(), ctx.File, ctx.Idx) {
			if d.FactoryName != "UNREACHABLE_CODE" || d.Line <= 0 || d.Col <= 0 || resolver.SmartCastCrossesModules(ctx.File.Path, d) {
				continue
			}
			if !unreachableHasSyntacticCause(ctx, resolver, d) {
				continue
			}
			ctx.EmitAt(d.Line, d.Col, "Unreachable code. Remove the statements that cannot execute.")
		}
		return
	}
	ctx.File.FlatWalkNodes(ctx.Idx, "jump_expression", func(jump uint32) { r.checkJump(ctx, jump) })
}

func (r *PrecompileUnreachableCodeRule) checkJump(ctx *api.Context, idx uint32) {
	file := ctx.File
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "statements" {
		return
	}
	keyword := jumpExpressionKeyword(file, idx)
	if keyword == "" {
		return
	}
	next := firstNamedStatementSiblingAfter(file, idx)
	if next == 0 {
		return
	}
	ctx.EmitAt(file.FlatRow(next)+1, file.FlatCol(next)+1,
		"Unreachable code after `"+keyword+"`. Remove the statements following the unconditional jump.")
}

func jumpExpressionKeyword(file *scanner.File, idx uint32) string {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		switch t {
		case "return", "throw", "break", "continue":
			return t
		}
	}
	return ""
}

func firstNamedStatementSiblingAfter(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatNextSib(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "line_comment", "multiline_comment":
			continue
		case "function_declaration", "class_declaration", "object_declaration", "type_alias":
			// A local declaration is not an executable statement, so kotlinc does not report it.
			continue
		}
		return child
	}
	return 0
}

// unreachableHasSyntacticCause keeps a compiler UNREACHABLE_CODE verdict only when the statement before the dead
// code visibly never completes: a jump, a branch construct whose arms all jump, `while (true)` or a call the
// compiler typed as Nothing. A type it could not resolve (a dependency that was not built) becomes an error type that
// the compiler treats as Nothing, which marks everything after it dead without any real jump.
func unreachableHasSyntacticCause(ctx *api.Context, resolver *oracle.CompositeResolver, d oracle.Diagnostic) bool {
	file := ctx.File
	node := nodeAtPoint(file, d.Line, d.Col-1)
	if node == 0 {
		return false
	}
	// A zero-width lookup at a block's first statement returns the block, so step into the statement.
	offset := uint32(file.LineOffset(d.Line-1) + d.Col - 1)
	for file.FlatType(node) == "statements" {
		inner := uint32(0)
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) && file.FlatStartByte(child) <= offset && offset < file.FlatEndByte(child) {
				inner = child
				break
			}
		}
		if inner == 0 {
			return false
		}
		node = inner
	}
	for {
		parent, ok := file.FlatParent(node)
		if !ok {
			return false
		}
		if file.FlatType(parent) == "statements" {
			break
		}
		node = parent
	}
	if file.FlatType(node) == "jump_expression" && jumpOperandNeverCompletes(file, resolver, node) {
		return true
	}
	for prev, ok := file.FlatPrevSibling(node); ok; prev, ok = file.FlatPrevSibling(prev) {
		if !file.FlatIsNamed(prev) {
			continue
		}
		switch file.FlatType(prev) {
		case "line_comment", "multiline_comment":
			continue
		}
		return statementNeverCompletes(file, resolver, prev)
	}
	return false
}

func jumpOperandNeverCompletes(file *scanner.File, resolver *oracle.CompositeResolver, jump uint32) bool {
	for child := file.FlatFirstChild(jump); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) && file.FlatType(child) != "label" && statementNeverCompletes(file, resolver, child) {
			return true
		}
	}
	return false
}

// statementNeverCompletes reports whether control cannot leave the node normally. An elvis only counts its left
// side: its right side runs only when the left is null.
func statementNeverCompletes(file *scanner.File, resolver *oracle.CompositeResolver, idx uint32) bool {
	switch file.FlatType(idx) {
	case "jump_expression":
		return true
	case "property_declaration", "parenthesized_expression", "assignment", "directly_assignable_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) && statementNeverCompletes(file, resolver, child) {
				return true
			}
		}
	case "elvis_expression":
		first := file.FlatFirstChild(idx)
		for first != 0 && !file.FlatIsNamed(first) {
			first = file.FlatNextSib(first)
		}
		return first != 0 && statementNeverCompletes(file, resolver, first)
	case "statements", "control_structure_body", "catch_block", "finally_block":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) && statementNeverCompletes(file, resolver, child) {
				return true
			}
		}
	case "if_expression":
		bodies := 0
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "control_structure_body" {
				bodies++
				if !statementNeverCompletes(file, resolver, child) {
					return false
				}
			}
		}
		return bodies == 2
	case "when_expression":
		hasElse := false
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "when_entry" {
				continue
			}
			if !file.FlatHasChildOfType(child, "when_condition") {
				hasElse = true
			}
			body, ok := file.FlatFindChild(child, "control_structure_body")
			if !ok || !statementNeverCompletes(file, resolver, body) {
				return false
			}
		}
		return hasElse
	case "try_expression":
		return tryNeverCompletes(file, resolver, idx)
	case "while_statement", "do_while_statement":
		return infiniteLoop(file, idx)
	case "call_expression", "navigation_expression":
		return callTypedNothing(file, resolver, idx)
	}
	return false
}

func tryNeverCompletes(file *scanner.File, resolver *oracle.CompositeResolver, idx uint32) bool {
	body := file.FlatFirstChild(idx)
	for body != 0 && file.FlatType(body) != "statements" {
		if t := file.FlatType(body); t == "catch_block" || t == "finally_block" {
			break
		}
		body = file.FlatNextSib(body)
	}
	bodyEnds := body != 0 && file.FlatType(body) == "statements" && statementNeverCompletes(file, resolver, body)
	catches := 0
	allCatchesEnd := true
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "catch_block":
			catches++
			if !statementNeverCompletes(file, resolver, child) {
				allCatchesEnd = false
			}
		case "finally_block":
			if statementNeverCompletes(file, resolver, child) {
				return true
			}
		}
	}
	return bodyEnds && allCatchesEnd
}

// infiniteLoop reports a `while (true)` loop with no break.
func infiniteLoop(file *scanner.File, idx uint32) bool {
	cond := file.FlatFirstChild(idx)
	for cond != 0 && !file.FlatIsNamed(cond) {
		cond = file.FlatNextSib(cond)
	}
	if cond == 0 || file.FlatType(cond) != "boolean_literal" || file.FlatNodeText(cond) != "true" {
		return false
	}
	hasBreak := false
	file.FlatWalkNodes(idx, "jump_expression", func(j uint32) {
		if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(j)), "break") {
			hasBreak = true
		}
	})
	return !hasBreak
}

// callTypedNothing reports whether the compiler resolved the call and typed it kotlin.Nothing.
func callTypedNothing(file *scanner.File, resolver *oracle.CompositeResolver, idx uint32) bool {
	rt := resolver.OracleExpressionFlat(file, idx)
	return rt != nil && rt.Resolved && (rt.FQN == "kotlin.Nothing" || rt.Kind == typeinfer.TypeNothing)
}
