package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

func flatEnclosingAncestor(file *scanner.File, idx uint32, types ...string) (uint32, bool) {
	if file == nil || len(types) == 0 {
		return 0, false
	}
	wants := make(map[string]struct{}, len(types))
	for _, t := range types {
		wants[t] = struct{}{}
	}
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if _, ok := wants[file.FlatType(current)]; ok {
			return current, true
		}
	}
	return 0, false
}

func flatEnclosingFunction(file *scanner.File, idx uint32) (uint32, bool) {
	return flatEnclosingAncestor(file, idx, "function_declaration")
}

func flatLastNamedChild(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatNamedChildCount(idx) == 0 {
		return 0
	}
	return file.FlatNamedChild(idx, file.FlatNamedChildCount(idx)-1)
}

func flatLastChildOfType(file *scanner.File, idx uint32, childType string) uint32 {
	if file == nil || idx == 0 {
		return 0
	}
	var last uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == childType {
			last = child
		}
	}
	return last
}

func flatUnwrapParenExpr(file *scanner.File, idx uint32) uint32 {
	for idx != 0 && file.FlatType(idx) == "parenthesized_expression" && file.FlatNamedChildCount(idx) > 0 {
		idx = file.FlatNamedChild(idx, 0)
	}
	return idx
}

// isBooleanLiteralTrue returns true if the node is a boolean_literal whose
// token is `true`. Accepts 0 and returns false (caller convenience).
func isBooleanLiteralTrue(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "boolean_literal" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "true" {
			return true
		}
	}
	return false
}

// ifExpressionHasElse returns true when an if_expression has an `else`
// token child. An if_expression with only the then branch has children
// (if, "(", condition, ")", control_structure_body); adding an else
// inserts an `else` keyword followed by a second control_structure_body.
func ifExpressionHasElse(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "if_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "else" {
			return true
		}
	}
	return false
}

// infixOperatorIs returns true when the infix_expression's middle
// simple_identifier equals `op`. Kotlin models `a to b`, `a shl b`,
// etc. as infix_expression with children (left, simple_identifier("op"),
// right).
func infixOperatorIs(file *scanner.File, idx uint32, op string) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "infix_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" && file.FlatNodeText(child) == op {
			return true
		}
	}
	return false
}

// androidResourceTypes is the allow-list of Android R-class resource
// directories recognized by naming-convention rules.
var androidResourceTypes = map[string]bool{
	"layout": true, "drawable": true, "string": true, "color": true,
	"dimen": true, "style": true, "menu": true, "anim": true,
	"xml": true, "raw": true, "id": true,
}
