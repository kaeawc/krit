package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// commitOrApplyNames is the set of callees that finalize a
// SharedPreferences.Editor chain.
var commitOrApplyNames = map[string]bool{
	"commit": true,
	"apply":  true,
}

// editorFinalizeCallShape returns true when call is a no-arg, no-trailing-lambda
// invocation of `commit` or `apply` — the shape that finalizes a
// SharedPreferences.Editor chain. Used to filter out scope-function and
// builder lookalikes from real Editor finalization.
func editorFinalizeCallShape(file *scanner.File, call uint32) bool {
	name := flatCallExpressionName(file, call)
	if name != "commit" && name != "apply" {
		return false
	}
	if flatCallTrailingLambda(file, call) != 0 {
		return false
	}
	args := flatCallKeyArguments(file, call)
	return args == 0 || file.FlatNamedChildCount(args) == 0
}

// ancestorFinalizesEditor walks upward from idx looking for an enclosing
// call_expression whose shape finalizes a SharedPreferences.Editor chain
// (no-arg `commit()` / `apply()`). Stops at function or source boundaries.
func ancestorFinalizesEditor(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression":
			if editorFinalizeCallShape(file, parent) {
				return true
			}
		case "function_declaration", "source_file":
			return false
		}
	}
	return false
}

// editorChainEscapes reports whether the expression chain that starts at idx
// (the `edit()` call and any fluent calls on it) is returned, passed as an
// argument or stored, so another owner is responsible for finalizing it.
func editorChainEscapes(file *scanner.File, idx uint32) bool {
	top := idx
	for parent, ok := file.FlatParent(top); ok; parent, ok = file.FlatParent(top) {
		switch file.FlatType(parent) {
		case "navigation_expression", "call_expression", "call_suffix", "navigation_suffix", "parenthesized_expression":
			top = parent
			continue
		}
		switch file.FlatType(parent) {
		case "jump_expression", "value_argument", "function_body":
			return true
		case "assignment":
			return assignmentRHS(file, parent) == top
		}
		return false
	}
	return false
}

// chainRootName returns the simple identifier at the start of a call chain such as `e.putInt(1).apply()`.
func chainRootName(file *scanner.File, call uint32) string {
	cur := call
	for cur != 0 {
		switch file.FlatType(cur) {
		case "simple_identifier":
			return file.FlatNodeText(cur)
		case "call_expression", "navigation_expression", "parenthesized_expression":
			cur = file.FlatNamedChild(cur, 0)
		default:
			return ""
		}
	}
	return ""
}

// variableEscapes reports whether the local named name is later returned, passed as an argument or assigned elsewhere inside fn.
func variableEscapes(file *scanner.File, fn uint32, name string) bool {
	escapes := false
	file.FlatWalkNodes(fn, "simple_identifier", func(id uint32) {
		if escapes || !file.FlatNodeTextEquals(id, name) {
			return
		}
		parent, ok := file.FlatParent(id)
		if !ok {
			return
		}
		switch file.FlatType(parent) {
		case "jump_expression", "value_argument", "function_body":
			escapes = true
		case "assignment":
			escapes = assignmentRHS(file, parent) == id
		}
	})
	return escapes
}
