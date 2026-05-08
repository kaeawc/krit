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
