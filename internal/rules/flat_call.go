package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func flatCallExpressionParts(file *scanner.File, idx uint32) (navExpr uint32, args uint32) {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return 0, 0
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_expression":
			navExpr = child
		case "value_arguments":
			args = child
		case "call_suffix":
			if args == 0 {
				args, _ = file.FlatFindChild(child, "value_arguments")
			}
		}
	}
	return navExpr, args
}

func flatCallExpressionName(file *scanner.File, idx uint32) string {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			return file.FlatNodeString(child, nil)
		case "navigation_expression":
			if name := flatNavigationExpressionLastIdentifier(file, child); name != "" {
				return name
			}
		}
	}
	return ""
}

func flatCallExpressionNameEquals(file *scanner.File, idx uint32, want string) bool {
	if file == nil || file.FlatType(idx) != "call_expression" || want == "" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			return file.FlatNodeTextEquals(child, want)
		case "navigation_expression":
			return flatNavigationExpressionLastIdentifierEquals(file, child, want)
		}
	}
	return false
}

// flatCallNameAny returns the method name of a call_expression whether the
// call uses the direct form (`name(args)`, `name { body }`) or the
// trailing-lambda idiom where tree-sitter nests the arg-ful call under an
// outer call_expression (`name(args) { body }` → outer call whose first
// child is the inner `name(args)` call_expression).
func flatCallNameAny(file *scanner.File, idx uint32) string {
	if name := flatCallExpressionName(file, idx); name != "" {
		return name
	}
	// Trailing-lambda outer call: recurse into a nested inner call_expression.
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "call_expression" {
			return flatCallExpressionName(file, child)
		}
	}
	return ""
}

// flatCallTrailingLambda returns the lambda_literal idx of the trailing
// lambda attached to a call_expression, or 0. Handles both `name { body }`
// and `name(args) { body }`.
func flatCallTrailingLambda(file *scanner.File, idx uint32) uint32 {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return 0
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "call_suffix" {
			continue
		}
		for sub := file.FlatFirstChild(child); sub != 0; sub = file.FlatNextSib(sub) {
			switch file.FlatType(sub) {
			case "annotated_lambda":
				if lit, ok := file.FlatFindChild(sub, "lambda_literal"); ok {
					return lit
				}
			case "lambda_literal":
				return sub
			}
		}
	}
	return 0
}

// flatCallKeyArguments returns the value_arguments idx of a call_expression
// — the positional/named argument list — or 0 if the call has no arguments.
// Handles both direct calls and trailing-lambda outer calls (where the args
// live on the nested inner call).
func flatCallKeyArguments(file *scanner.File, idx uint32) uint32 {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return 0
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "call_suffix":
			if va, ok := file.FlatFindChild(child, "value_arguments"); ok {
				return va
			}
		case "call_expression":
			// Trailing-lambda idiom: the args live on the nested inner call.
			if suffix, ok := file.FlatFindChild(child, "call_suffix"); ok {
				if va, ok := file.FlatFindChild(suffix, "value_arguments"); ok {
					return va
				}
			}
		}
	}
	return 0
}

func flatCallForwardsEnclosingFunctionParameters(file *scanner.File, call, args uint32) bool {
	if file == nil || args == 0 {
		return false
	}
	fn, ok := flatEnclosingAncestor(file, call, "function_declaration")
	if !ok {
		return false
	}
	params := flatFunctionParameterNames(file, fn)
	if len(params) == 0 {
		return false
	}
	paramNames := make(map[string]bool, len(params))
	for _, name := range params {
		paramNames[name] = true
	}
	total := 0
	forwarded := 0
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "value_argument" {
			continue
		}
		total++
		text := strings.TrimSpace(file.FlatNodeText(child))
		if paramNames[text] {
			forwarded++
		}
	}
	return total > 0 && forwarded >= len(params) && forwarded >= total-1
}

func flatReceiverNameFromCall(file *scanner.File, idx uint32) string {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return ""
	}
	first := file.FlatNamedChild(navExpr, 0)
	switch file.FlatType(first) {
	case "simple_identifier":
		return file.FlatNodeString(first, nil)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, first)
	default:
		return ""
	}
}

func flatNamedValueArgument(file *scanner.File, args uint32, name string) uint32 {
	if file == nil || args == 0 {
		return 0
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if flatValueArgumentLabel(file, arg) == name {
			return arg
		}
	}
	return 0
}

func flatPositionalValueArgument(file *scanner.File, args uint32, index int) uint32 {
	if file == nil || args == 0 || index < 0 {
		return 0
	}
	current := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if flatHasValueArgumentLabel(file, arg) {
			continue
		}
		if current == index {
			return arg
		}
		current++
	}
	return 0
}

func flatValueArgumentLabel(file *scanner.File, arg uint32) string {
	if file == nil || arg == 0 {
		return ""
	}
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "value_argument_label":
			text := strings.TrimSpace(file.FlatNodeText(child))
			return strings.TrimSuffix(text, "=")
		case "simple_identifier":
			if next, ok := file.FlatNextSibling(child); ok && file.FlatType(next) == "=" {
				return file.FlatNodeString(child, nil)
			}
		}
	}
	return ""
}

func flatHasValueArgumentLabel(file *scanner.File, arg uint32) bool {
	if file == nil || arg == 0 {
		return false
	}
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "value_argument_label":
			return true
		case "simple_identifier":
			if next, ok := file.FlatNextSibling(child); ok && file.FlatType(next) == "=" {
				return true
			}
		}
	}
	return false
}

func flatValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	if file == nil || arg == 0 || file.FlatNamedChildCount(arg) == 0 {
		return 0
	}
	afterEquals := false
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			afterEquals = true
			continue
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if afterEquals {
			return child
		}
		if file.FlatType(child) == "value_argument_label" {
			continue
		}
		if next, ok := file.FlatNextSibling(child); ok && file.FlatType(next) == "=" {
			continue
		}
		return child
	}
	return 0
}

func flatCallSuffixLambdaNode(file *scanner.File, suffix uint32) uint32 {
	if file == nil || suffix == 0 {
		return 0
	}
	if lambda, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
		if lit, ok := file.FlatFindChild(lambda, "lambda_literal"); ok {
			return lit
		}
		return lambda
	}
	if lambda, ok := file.FlatFindChild(suffix, "lambda_literal"); ok {
		return lambda
	}
	return 0
}

func flatCallSuffixHasArgs(file *scanner.File, suffix uint32) bool {
	if file == nil || suffix == 0 {
		return false
	}
	args, _ := file.FlatFindChild(suffix, "value_arguments")
	return args != 0 && file.FlatNamedChildCount(args) > 0
}

func flatCallSuffixValueArgs(file *scanner.File, suffix uint32) uint32 {
	if file == nil || suffix == 0 {
		return 0
	}
	if args, ok := file.FlatFindChild(suffix, "value_arguments"); ok {
		return args
	}
	return 0
}

// showCallName is a single-entry set for the `show` callee — used by
// rules that want to check whether a fluent builder was ever "shown".
var showCallName = map[string]bool{"show": true}

// logMethodNames is the set of android.util.Log level methods. `wtf` is
// included for completeness even though the AOSP tag rules historically
// only covered v/d/i/w/e.
var logMethodNames = map[string]bool{
	"v": true, "d": true, "i": true, "w": true, "e": true, "wtf": true,
}

// composeSemanticsEscapeHatches are the Compose call names that opt an
// otherwise-accessible component OUT of TalkBack exposure. Used by
// decorative-image a11y rules to skip calls that already declare the
// intent.
var composeSemanticsEscapeHatches = map[string]bool{
	"clearAndSetSemantics": true,
	"invisibleToUser":      true,
}

// channelCloseNames is the set of Channel finalization methods that
// release the receiver coroutine.
var channelCloseNames = map[string]bool{"close": true}

// coroutineScopeCancelNames is the set of CoroutineScope finalization
// methods that stop the launched coroutines.
var coroutineScopeCancelNames = map[string]bool{"cancel": true}

// commitTransactionNames is the set of callees that finalize a
// FragmentTransaction chain.
var commitTransactionNames = map[string]bool{
	"commit":                     true,
	"commitNow":                  true,
	"commitAllowingStateLoss":    true,
	"commitNowAllowingStateLoss": true,
}

// checkResultCalleeNames is the set of callees whose return value
// should almost never be discarded (idempotent builders, pure string
// operations, animator configurers that return `this`).
var checkResultCalleeNames = map[string]bool{
	"animate":   true,
	"buildUpon": true,
	"edit":      true,
	"format":    true,
	"trim":      true,
	"replace":   true,
}

// isReceiverNamed returns true when the call_expression at idx is invoked
// through a navigation whose first identifier equals `name`. Matches
// `Toast.makeText(...)` when called with `name == "Toast"`. Returns false
// for nested qualifications like `a.b.makeText(...)` — only flat
// qualification counts.
func isReceiverNamed(file *scanner.File, idx uint32, name string) bool {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return false
	}
	first := file.FlatFirstChild(navExpr)
	for first != 0 && !file.FlatIsNamed(first) {
		first = file.FlatNextSib(first)
	}
	return first != 0 && file.FlatType(first) == "simple_identifier" && file.FlatNodeText(first) == name
}

// ancestorCallNameMatches walks upward looking for an enclosing
// call_expression whose callee equals `name`. Used to detect the
// fluent chain pattern `foo(...).show()` — where the inner call is
// idx and the outer call_expression calls `show`.
func ancestorCallNameMatches(file *scanner.File, idx uint32, name string) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression":
			if flatCallExpressionName(file, parent) == name {
				return true
			}
		case "function_declaration", "source_file":
			return false
		}
	}
	return false
}

// callSubtreeHasNamedArgument returns true when any call_expression in
// the subtree rooted at `root` (including root itself) has a named
// value_argument with the given name. Used to check whether a nested
// call in a trailing lambda supplies an argument that the outer call
// appears to be missing.
func callSubtreeHasNamedArgument(file *scanner.File, root uint32, name string) bool {
	if file == nil || root == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(root, "call_expression", func(call uint32) {
		if found {
			return
		}
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return
		}
		if flatNamedValueArgument(file, args, name) != 0 {
			found = true
		}
	})
	return found
}

// classHasCallOn returns true when some call_expression inside the
// class at classIdx has a receiver identifier equal to receiverName
// and a callee name in the given set. Used by rules that want to
// verify "did this class ever invoke X on property Y?" without
// string-matching the class body text.
func classHasCallOn(file *scanner.File, classIdx uint32, receiverName string, calleeNames map[string]bool) bool {
	if file == nil || classIdx == 0 || receiverName == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(classIdx, "call_expression", func(call uint32) {
		if found {
			return
		}
		if !calleeNames[flatCallExpressionName(file, call)] {
			return
		}
		if flatReceiverNameFromCall(file, call) == receiverName {
			found = true
		}
	})
	return found
}

// namedArgRHSIsNullLiteral returns true when the RHS of a named
// value_argument (`name = <expr>`) is the bare `null` keyword. Unlike
// flatValueArgumentExpression, this walks past unnamed token children
// so that tree-sitter's `null` keyword (which is not a "named" node) is
// detected.
func namedArgRHSIsNullLiteral(file *scanner.File, arg uint32) bool {
	if file == nil || arg == 0 {
		return false
	}
	seenEquals := false
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if !seenEquals {
			continue
		}
		if file.FlatType(child) == "null" {
			return true
		}
		// Any other non-trivia child means the RHS is a concrete expression
		// (not null) — bail out.
		if file.FlatIsNamed(child) {
			return false
		}
	}
	return false
}

// subtreeHasCalleeIn walks every call_expression inside `root` and
// returns true when any callee name is in `names`. The root node itself
// is excluded — a call's callee is not "inside" the call. Callers use
// this to ask "does this call's arguments or trailing lambda invoke any
// of these APIs?".
func subtreeHasCalleeIn(file *scanner.File, root uint32, names map[string]bool) bool {
	if file == nil || root == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(root, "call_expression", func(call uint32) {
		if found || call == root {
			return
		}
		if names[flatCallExpressionName(file, call)] {
			found = true
		}
	})
	return found
}

func ancestorCallNameIn(file *scanner.File, idx uint32, names map[string]bool) bool {
	if file == nil || idx == 0 {
		return false
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression":
			if names[flatCallExpressionName(file, parent)] {
				return true
			}
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return false
		}
	}
	return false
}

// functionHasReceiverCallAfter reports whether fn contains a call after target
// whose receiver name and callee match. accept can reject same-name calls that
// are not the API being searched for, such as Kotlin scope-function apply.
func functionHasReceiverCallAfter(file *scanner.File, fn, target uint32, receiverName string, names map[string]bool, accept func(*scanner.File, uint32) bool) bool {
	if file == nil || fn == 0 || target == 0 || receiverName == "" {
		return false
	}
	targetStart := file.FlatStartByte(target)
	found := false
	file.FlatWalkNodes(fn, "call_expression", func(call uint32) {
		if found || call == target || file.FlatStartByte(call) < targetStart {
			return
		}
		if !names[flatCallExpressionName(file, call)] {
			return
		}
		if flatReceiverNameFromCall(file, call) != receiverName {
			return
		}
		if accept != nil && !accept(file, call) {
			return
		}
		found = true
	})
	return found
}
