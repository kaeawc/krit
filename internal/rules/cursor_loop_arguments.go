package rules

import "github.com/kaeawc/krit/internal/scanner"

func cursorColumnArgumentInvariant(file *scanner.File, call, body, args uint32, locals map[string]bool) bool {
	if args == 0 || file.FlatNamedChildCount(args) != 1 {
		return false
	}
	expr := file.FlatNamedChild(args, 0)
	if file.FlatType(expr) == "value_argument" {
		expr = flatValueArgumentExpression(file, expr)
	}
	if cursorPlainString(file, expr) {
		return true
	}
	if cursorPlatformColumn(file, call, expr) {
		return true
	}
	kind := file.FlatType(expr)
	if kind != "simple_identifier" && kind != "identifier" {
		return false
	}
	name := file.FlatNodeText(expr)
	return !locals[name] && cursorBindingInvariant(file, call, body, name, true)
}

func cursorPlainString(file *scanner.File, expr uint32) bool {
	return file.FlatType(expr) == "string_literal" && !flatContainsStringInterpolation(file, expr)
}
