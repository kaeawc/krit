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
