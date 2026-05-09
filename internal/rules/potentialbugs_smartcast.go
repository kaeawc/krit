package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// SmartCastInvalidatedRule — flags reassignment of a smart-cast variable
// followed by an unsafe navigation/call use of the same name within the same
// if-body scope. Mimics kotlinc's SMARTCAST_IMPOSSIBLE for the local-`var`
// pattern:
//
//	var x: String? = "a"
//	if (x != null) {
//	    x = bar()
//	    println(x.length)  // SMARTCAST_IMPOSSIBLE
//	}
//
// The rule is intentionally narrow:
//   - Only fires when the variable is declared with `var` somewhere in the
//     enclosing function body (parameters and properties are out of scope —
//     parameters can't be reassigned in Kotlin, and class properties have
//     different rules the resolver tracks).
//   - Only handles `if (x != null)` (and `if (x == null) return` early-exit
//     is not considered here because reassignment after the early return
//     would not invalidate a smart cast).
//   - Only flags the first reassignment-then-use pair to keep messages tight.
//
// ---------------------------------------------------------------------------
type SmartCastInvalidatedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SmartCastInvalidatedRule) Confidence() float64 { return 0.85 }

// smartCastIfNullCheckedVar returns the simple identifier name being
// non-null-checked by the if's condition, or "" when the shape doesn't
// match `x != null`.
func smartCastIfNullCheckedVar(file *scanner.File, ifIdx uint32) string {
	if file == nil || ifIdx == 0 || file.FlatType(ifIdx) != "if_expression" {
		return ""
	}
	var cond uint32
	for child := file.FlatFirstChild(ifIdx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "(" {
			continue
		}
		if file.FlatIsNamed(child) {
			cond = child
			break
		}
	}
	if cond == 0 {
		return ""
	}
	eq := cond
	if file.FlatType(eq) != "equality_expression" {
		// Some grammars wrap; descend one level.
		for child := file.FlatFirstChild(cond); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "equality_expression" {
				eq = child
				break
			}
		}
	}
	if file.FlatType(eq) != "equality_expression" {
		return ""
	}
	left, op, right := smartCastEqualityOperands(file, eq)
	if left == 0 || op == 0 || right == 0 {
		return ""
	}
	if strings.TrimSpace(file.FlatNodeText(op)) != "!=" {
		return ""
	}
	leftText := strings.TrimSpace(file.FlatNodeText(left))
	rightText := strings.TrimSpace(file.FlatNodeText(right))
	if rightText == "null" && file.FlatType(left) == "simple_identifier" {
		return leftText
	}
	if leftText == "null" && file.FlatType(right) == "simple_identifier" {
		return rightText
	}
	// Tree-sitter parses `null` as a token node whose own type is "null"
	// rather than a simple_identifier; the simple-text check above already
	// handles that side, but include the explicit type-name match too in
	// case the operand is a literal node rather than an identifier.
	if file.FlatType(right) == "null" && file.FlatType(left) == "simple_identifier" {
		return leftText
	}
	if file.FlatType(left) == "null" && file.FlatType(right) == "simple_identifier" {
		return rightText
	}
	return ""
}

func smartCastEqualityOperands(file *scanner.File, idx uint32) (left, op, right uint32) {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "==", "!=", "===", "!==":
			op = child
			continue
		}
		// Both operands include the `null` literal which tree-sitter
		// emits as a non-named token node ("null" type). Don't filter
		// on FlatIsNamed here; instead skip only the parens that wrap
		// the equality_expression in some grammars.
		if t := file.FlatType(child); t == "(" || t == ")" {
			continue
		}
		if op == 0 {
			left = child
		} else if right == 0 {
			right = child
		}
	}
	return left, op, right
}

// smartCastIfThenBody returns the control_structure_body of the if's
// then-branch, or 0 when none.
func smartCastIfThenBody(file *scanner.File, ifIdx uint32) uint32 {
	for child := file.FlatFirstChild(ifIdx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "control_structure_body" {
			return child
		}
	}
	return 0
}

// smartCastFunctionDeclaresVar reports whether the enclosing function body
// (or the source file when no enclosing function) declares a local
// `var <varName>`. Looks for property_declaration nodes whose first child
// is the `var` keyword and whose variable_declaration name matches.
func smartCastFunctionDeclaresVar(file *scanner.File, scopeIdx uint32, varName string) bool {
	if file == nil || scopeIdx == 0 || varName == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(scopeIdx, func(n uint32) {
		if found {
			return
		}
		if file.FlatType(n) != "property_declaration" {
			return
		}
		// Tree-sitter wraps the `var`/`val` keyword in a
		// binding_pattern_kind node. Descend to find the actual keyword.
		isVar := false
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "binding_pattern_kind" {
				for kw := file.FlatFirstChild(child); kw != 0; kw = file.FlatNextSib(kw) {
					if file.FlatType(kw) == "var" {
						isVar = true
						break
					}
					if file.FlatType(kw) == "val" {
						return
					}
				}
				break
			}
			if file.FlatType(child) == "var" {
				isVar = true
				break
			}
			if file.FlatType(child) == "val" {
				return
			}
		}
		if !isVar {
			return
		}
		varDecl, _ := file.FlatFindChild(n, "variable_declaration")
		if varDecl == 0 {
			return
		}
		nameNode, _ := file.FlatFindChild(varDecl, "simple_identifier")
		if nameNode == 0 {
			return
		}
		if strings.TrimSpace(file.FlatNodeText(nameNode)) == varName {
			found = true
		}
	})
	return found
}

// smartCastAssignmentTargetName returns the simple identifier name of
// an assignment's LHS, walking the directly_assignable_expression
// wrapper that tree-sitter emits. Returns "" when the LHS isn't a bare
// identifier (e.g. property access or array index — those aren't local
// var reassignments and don't invalidate a smart cast on the same name
// in the way this rule cares about).
func smartCastAssignmentTargetName(file *scanner.File, assign uint32) string {
	if file == nil || assign == 0 {
		return ""
	}
	for child := file.FlatFirstChild(assign); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			return ""
		}
		switch file.FlatType(child) {
		case "simple_identifier":
			return strings.TrimSpace(file.FlatNodeText(child))
		case "directly_assignable_expression":
			// Descend; the inner simple_identifier is the bare name.
			for inner := file.FlatFirstChild(child); inner != 0; inner = file.FlatNextSib(inner) {
				if file.FlatType(inner) == "simple_identifier" {
					return strings.TrimSpace(file.FlatNodeText(inner))
				}
				if file.FlatIsNamed(inner) {
					// Not a bare identifier — bail.
					return ""
				}
			}
			return ""
		}
	}
	return ""
}

// smartCastFindReassignment returns the byte offset of the first
// assignment whose LHS is `varName` inside body, or 0 when none exists.
// Scans `assignment` nodes; an `assignment` whose first named child is a
// simple_identifier with the matching name counts. Compound assignments
// (`+=`, `-=`, etc.) are also assignment nodes in tree-sitter Kotlin.
func smartCastFindReassignment(file *scanner.File, body uint32, varName string) uint32 {
	if file == nil || body == 0 || varName == "" {
		return 0
	}
	var first uint32
	file.FlatWalkAllNodes(body, func(n uint32) {
		if first != 0 {
			return
		}
		if file.FlatType(n) != "assignment" {
			return
		}
		lhs := smartCastAssignmentTargetName(file, n)
		if lhs != varName {
			return
		}
		first = file.FlatStartByte(n)
	})
	return first
}

// smartCastFindUseAfter returns (row, col) of the first navigation_expression
// or call_expression whose receiver is `varName` and whose start byte is
// after `afterByte`, or (0, 0) when none.
func smartCastFindUseAfter(file *scanner.File, body uint32, varName string, afterByte uint32) (int, int) {
	if file == nil || body == 0 || varName == "" {
		return 0, 0
	}
	var row, col int
	file.FlatWalkAllNodes(body, func(n uint32) {
		if row != 0 {
			return
		}
		if file.FlatStartByte(n) <= afterByte {
			return
		}
		if file.FlatType(n) != "navigation_expression" {
			return
		}
		recv := file.FlatFirstChild(n)
		for recv != 0 && !file.FlatIsNamed(recv) {
			recv = file.FlatNextSib(recv)
		}
		if recv == 0 || file.FlatType(recv) != "simple_identifier" {
			return
		}
		if strings.TrimSpace(file.FlatNodeText(recv)) != varName {
			return
		}
		// Skip safe-call (`?.`) — those are explicitly not relying on a
		// smart cast.
		text := file.FlatNodeText(n)
		if strings.Contains(text, "?.") {
			return
		}
		row = file.FlatRow(n) + 1
		col = file.FlatCol(n) + 1
	})
	return row, col
}

func (r *SmartCastInvalidatedRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	varName := smartCastIfNullCheckedVar(file, idx)
	if varName == "" {
		return
	}
	body := smartCastIfThenBody(file, idx)
	if body == 0 {
		return
	}
	fn, ok := flatEnclosingAncestor(file, idx, "function_declaration")
	if !ok {
		return
	}
	if !smartCastFunctionDeclaresVar(file, fn, varName) {
		return
	}
	reassignByte := smartCastFindReassignment(file, body, varName)
	if reassignByte == 0 {
		return
	}
	row, col := smartCastFindUseAfter(file, body, varName, reassignByte)
	if row == 0 {
		return
	}
	ctx.EmitAt(row, col,
		fmt.Sprintf("Smart cast on '%s' is invalidated: '%s' was reassigned earlier in this block; use a local copy or '?.' access.", varName, varName))
}
