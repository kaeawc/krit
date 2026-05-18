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

// localScopeBoundaryTypes lists node types that begin a new local
// execution context. A walker that wants to inspect only the same
// synchronous scope as `root` should refuse to descend into these
// subtrees:
//
//   - function_declaration: nested local funs run only when called.
//   - lambda_literal / anonymous_function: bodies run when invoked,
//     often on a different thread (coroutine builders, callbacks).
//   - class_declaration / object_declaration: nested type bodies and
//     `object : I { ... }` expressions execute their member code on
//     method invocation, not as part of the enclosing function.
//
// Sibling constructs that DO share the enclosing function's frame
// (init/`anonymous_initializer`, `secondary_constructor` calls into
// the primary, property getters/setters reachable through expressions)
// are intentionally absent so an in-scope cleanup is still seen.
var localScopeBoundaryTypes = map[string]struct{}{
	"function_declaration": {},
	"lambda_literal":       {},
	"anonymous_function":   {},
	"class_declaration":    {},
	"object_declaration":   {},
}

// flatWalkLocalScope visits every descendant of `root` that lives in
// the same local scope as `root` itself, stopping at any node listed
// in localScopeBoundaryTypes. The callback is invoked for `root` and
// for each in-scope descendant in pre-order. Use when a rule needs to
// reason about the synchronous execution of a function body — e.g.
// MDC.put/clear/remove pairing — without being misled by clean-up
// calls inside coroutine builders, callbacks, or local helper
// functions whose execution context is unknown.
func flatWalkLocalScope(file *scanner.File, root uint32, fn func(uint32)) {
	if file == nil || file.FlatTree == nil || root == 0 || fn == nil {
		return
	}
	var walk func(uint32)
	walk = func(idx uint32) {
		if idx != root {
			if _, isBoundary := localScopeBoundaryTypes[file.FlatType(idx)]; isBoundary {
				return
			}
		}
		fn(idx)
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			walk(child)
		}
	}
	walk(root)
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
