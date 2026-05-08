package astflat

import "github.com/kaeawc/krit/internal/scanner"

// CallExpressionParts splits a call_expression into its navigation_expression
// (the callee) and value_arguments (the args). Either return value may be 0
// when absent or when the call uses a trailing-lambda only.
func CallExpressionParts(file *scanner.File, idx uint32) (navExpr uint32, args uint32) {
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

// NavigationExpressionLastIdentifier returns the rightmost simple_identifier
// reachable inside a navigation_expression. For `a.b.c` this returns "c".
// Skips over value-arguments / lambda-literal / string-literal subtrees so
// argument identifiers don't shadow the actual callee name.
func NavigationExpressionLastIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	last := ""
	var walk func(uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "simple_identifier":
			last = file.FlatNodeString(n, nil)
			return
		case "value_arguments", "value_argument", "call_suffix", "lambda_literal", "string_literal":
			return
		}
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				walk(child)
			}
		}
	}
	walk(idx)
	return last
}

// NavigationExpressionReceiver returns the receiver (first named child) of a
// navigation_expression, or 0 if the node is malformed.
func NavigationExpressionReceiver(file *scanner.File, nav uint32) uint32 {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChildCount(nav) < 2 {
		return 0
	}
	return file.FlatNamedChild(nav, 0)
}

// NavigationLastSuffixHasSafeAccess reports whether the trailing
// navigation_suffix uses safe-call (`?.`) rather than `.`.
func NavigationLastSuffixHasSafeAccess(file *scanner.File, nav uint32) bool {
	var suffix uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "navigation_suffix" {
			suffix = child
		}
	}
	if suffix == 0 {
		return false
	}
	for child := file.FlatFirstChild(suffix); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatType(child) == "?." {
			return true
		}
	}
	return false
}

// CallExpressionName returns the rightmost identifier name of a
// call_expression's callee — either the simple_identifier or, for navigation
// callees, the last identifier in the chain.
func CallExpressionName(file *scanner.File, idx uint32) string {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			return file.FlatNodeString(child, nil)
		case "navigation_expression":
			if name := NavigationExpressionLastIdentifier(file, child); name != "" {
				return name
			}
		}
	}
	return ""
}

// CallNameAny is like CallExpressionName but also handles the trailing-lambda
// idiom where tree-sitter wraps `name(args) { body }` in an outer
// call_expression whose first child is the inner `name(args)` call.
func CallNameAny(file *scanner.File, idx uint32) string {
	if name := CallExpressionName(file, idx); name != "" {
		return name
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "call_expression" {
			return CallExpressionName(file, child)
		}
	}
	return ""
}

// CallSuffixValueArgs returns the value_arguments child of a call_suffix
// node, or 0.
func CallSuffixValueArgs(file *scanner.File, suffix uint32) uint32 {
	if file == nil || suffix == 0 {
		return 0
	}
	if args, ok := file.FlatFindChild(suffix, "value_arguments"); ok {
		return args
	}
	return 0
}

// HasValueArgumentLabel reports whether a value_argument has a named label
// (either via value_argument_label child or `name = ...` shape).
func HasValueArgumentLabel(file *scanner.File, arg uint32) bool {
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

// ValueArgumentExpression returns the expression node carried by a
// value_argument, skipping any leading label.
func ValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
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

// SingleValueArgumentExpression returns the lone value_argument expression in
// args, or (0, false) if the args list contains zero or more than one
// argument.
func SingleValueArgumentExpression(file *scanner.File, args uint32) (uint32, bool) {
	var arg uint32
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "value_argument" {
			continue
		}
		if arg != 0 {
			return 0, false
		}
		arg = child
	}
	if arg == 0 {
		return 0, false
	}
	if HasValueArgumentLabel(file, arg) {
		expr := LastNamedChild(file, arg)
		return expr, expr != 0
	}
	expr := ValueArgumentExpression(file, arg)
	return expr, expr != 0
}

// PositionalValueArgument returns the index-th positional (unlabeled)
// argument from a value_arguments list, or 0 if absent.
func PositionalValueArgument(file *scanner.File, args uint32, index int) uint32 {
	if file == nil || args == 0 || index < 0 {
		return 0
	}
	current := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if HasValueArgumentLabel(file, arg) {
			continue
		}
		if current == index {
			return arg
		}
		current++
	}
	return 0
}
