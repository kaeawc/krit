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

func flatNavigationExpressionLastIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	last := ""
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_suffix":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
					last = file.FlatNodeString(gc, nil)
				}
			}
		case "simple_identifier":
			last = file.FlatNodeString(child, nil)
		}
	}
	return last
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

// flatFunctionParameterNames returns the simple_identifier names of every
// parameter in a function_declaration's function_value_parameters block.
func flatFunctionParameterNames(file *scanner.File, funcDecl uint32) []string {
	if file == nil || file.FlatType(funcDecl) != "function_declaration" {
		return nil
	}
	params, _ := file.FlatFindChild(funcDecl, "function_value_parameters")
	if params == 0 {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		if ident, ok := file.FlatFindChild(child, "simple_identifier"); ok {
			names = append(names, file.FlatNodeString(ident, nil))
		}
	}
	return names
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

func flatLastNamedChild(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatNamedChildCount(idx) == 0 {
		return 0
	}
	return file.FlatNamedChild(idx, file.FlatNamedChildCount(idx)-1)
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

func flatContainsStringInterpolation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "interpolated_identifier", "interpolated_expression",
			"line_string_expression", "multi_line_string_expression",
			"line_str_ref", "multi_line_str_ref":
			found = true
		}
	})
	return found
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

// finalSimpleIdentifier returns the identifier text at the end of a
// navigation chain or directly_assignable_expression — the rightmost
// simple_identifier reachable by walking named children. For
// `w.settings.javaScriptEnabled` this returns `javaScriptEnabled`.
func finalSimpleIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	// Walk children looking for the last simple_identifier (direct or in a
	// navigation_suffix).
	last := ""
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			last = file.FlatNodeText(child)
		case "navigation_suffix":
			if inner, ok := file.FlatFindChild(child, "simple_identifier"); ok {
				last = file.FlatNodeText(inner)
			}
		case "navigation_expression", "directly_assignable_expression":
			if nested := finalSimpleIdentifier(file, child); nested != "" {
				last = nested
			}
		}
	}
	return last
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

// classOverriddenFunctions returns the set of function names declared
// with the `override` modifier at class-top-level. Used by rules that
// need to answer "did this subclass override method X?" without scanning
// source text for `override fun X(`.
func classOverriddenFunctions(file *scanner.File, classIdx uint32) map[string]bool {
	out := map[string]bool{}
	if file == nil || classIdx == 0 {
		return out
	}
	body, _ := file.FlatFindChild(classIdx, "class_body")
	if body == 0 {
		return out
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "function_declaration" {
			continue
		}
		if !file.FlatHasModifier(child, "override") {
			continue
		}
		ident, _ := file.FlatFindChild(child, "simple_identifier")
		if ident == 0 {
			continue
		}
		out[file.FlatNodeText(ident)] = true
	}
	return out
}

// classHasSupertypeNamed returns true when the class_declaration at idx
// lists a supertype whose final type_identifier equals `name`. Covers
// both interface form (`: Foo`, delegation_specifier→user_type) and
// class form with a constructor call (`: Foo()`,
// delegation_specifier→constructor_invocation→user_type). Works for
// qualified receivers like `: pkg.sub.Foo[()]`.
func classHasSupertypeNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "class_declaration" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		userType, _ := file.FlatFindChild(child, "user_type")
		if userType == 0 {
			// `: Foo(args)` form — user_type lives under constructor_invocation.
			if ctor, ok := file.FlatFindChild(child, "constructor_invocation"); ok {
				userType, _ = file.FlatFindChild(ctor, "user_type")
			}
		}
		if userType == 0 {
			continue
		}
		if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
			if file.FlatNodeText(ident) == name {
				return true
			}
		}
	}
	return false
}

// classDeclaresStaticProperty returns true when the class at idx declares
// a property named `name` at class-top-level or inside a companion_object
// (which is how Kotlin models static fields like Parcelable.CREATOR).
func classDeclaresStaticProperty(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 {
		return false
	}
	body, _ := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return false
	}
	if classBodyHasProperty(file, body, name) {
		return true
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "companion_object" {
			innerBody, _ := file.FlatFindChild(child, "class_body")
			if classBodyHasProperty(file, innerBody, name) {
				return true
			}
		}
	}
	return false
}

// classBodyHasProperty returns true when body contains a property_declaration
// whose variable_declaration's simple_identifier matches `name`.
func classBodyHasProperty(file *scanner.File, body uint32, name string) bool {
	if file == nil || body == 0 {
		return false
	}
	found := false
	for child := file.FlatFirstChild(body); child != 0 && !found; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "property_declaration" {
			continue
		}
		if propertyDeclarationName(file, child) == name {
			found = true
		}
	}
	return found
}

// propertyDeclarationName returns the identifier name of a property_declaration,
// or "" if the node isn't a property_declaration or has no variable_declaration.
func propertyDeclarationName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 || file.FlatType(idx) != "property_declaration" {
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

// commitOrApplyNames is the set of callees that finalize a
// SharedPreferences.Editor chain.
var commitOrApplyNames = map[string]bool{
	"commit": true,
	"apply":  true,
}

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

// enclosingFunctionHasCallNamed walks every call_expression under the
// given function container and returns true when any call OTHER than
// `except` has a callee in `names`. Used for "did the chain eventually
// finalize?" checks without touching node text.
func enclosingFunctionHasCallNamed(file *scanner.File, fn, except uint32, names map[string]bool) bool {
	if file == nil || fn == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(fn, "call_expression", func(call uint32) {
		if found || call == except {
			return
		}
		if names[flatCallExpressionName(file, call)] {
			found = true
		}
	})
	return found
}

// isReceiverString returns true when the call_expression at idx is
// invoked on the simple name `String` (e.g. `String.format(...)`).
func isReceiverString(file *scanner.File, idx uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return false
	}
	first := file.FlatFirstChild(navExpr)
	for first != 0 && !file.FlatIsNamed(first) {
		first = file.FlatNextSib(first)
	}
	return first != 0 && file.FlatType(first) == "simple_identifier" && file.FlatNodeText(first) == "String"
}

// hasAnnotationNamed returns true when the declaration at idx has a
// modifier-list annotation whose final name is exactly `name`. Checks
// both the declaration's `modifiers` child and its immediately preceding
// sibling (some grammar versions emit modifiers as a sibling node).
func hasAnnotationNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 {
		return false
	}
	check := func(container uint32) bool {
		if container == 0 {
			return false
		}
		found := false
		file.FlatWalkNodes(container, "annotation", func(ann uint32) {
			if found {
				return
			}
			ctor, _ := file.FlatFindChild(ann, "constructor_invocation")
			if ctor != 0 {
				if annotationConstructorName(file, ctor) == name {
					found = true
				}
				return
			}
			// Marker annotation (no constructor call): `@Foo`
			userType, _ := file.FlatFindChild(ann, "user_type")
			if userType != 0 {
				if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
					if file.FlatNodeText(ident) == name {
						found = true
					}
				}
			}
		})
		return found
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok && check(mods) {
		return true
	}
	if prev, ok := file.FlatPrevSibling(idx); ok && check(prev) {
		return true
	}
	return false
}

// annotationConstructorName returns the final identifier of an annotation's
// constructor_invocation. For `@foo.bar.IntDef(...)` this returns "IntDef".
func annotationConstructorName(file *scanner.File, ctor uint32) string {
	if file == nil || ctor == 0 {
		return ""
	}
	userType, _ := file.FlatFindChild(ctor, "user_type")
	if userType == 0 {
		return ""
	}
	ident := flatLastChildOfType(file, userType, "type_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

// annotationConstantKey returns (canonicalKey, display) for a constant
// appearing in an annotation argument. Supports numeric literals (key is
// the literal text), string literals (key is the interpolation-free
// content prefixed with `"s:"`), and simple identifiers / qualified
// navigation expressions (key is the dotted FQN prefixed with `"id:"`).
// Returns ("", "") for expressions we don't recognize as a simple
// constant reference.
func annotationConstantKey(file *scanner.File, expr uint32) (key, display string) {
	if file == nil || expr == 0 {
		return "", ""
	}
	switch file.FlatType(expr) {
	case "integer_literal", "long_literal", "hex_literal", "bin_literal":
		t := file.FlatNodeText(expr)
		return "n:" + t, t
	case "string_literal":
		if flatContainsStringInterpolation(file, expr) {
			return "", ""
		}
		c := stringLiteralContent(file, expr)
		return "s:" + c, `"` + c + `"`
	case "simple_identifier", "navigation_expression":
		t := strings.TrimSpace(file.FlatNodeText(expr))
		return "id:" + t, t
	}
	return "", ""
}

// assignmentRHS returns the expression on the right of `=` in an assignment.
func assignmentRHS(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatType(idx) != "assignment" {
		return 0
	}
	seenEquals := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if seenEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}
