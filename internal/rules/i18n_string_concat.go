package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// StringConcatForTranslationRule flags `+` concatenation between a
// `stringResource(...)` call and any non-literal operand. Such
// concatenation hard-codes English word order; translators cannot
// reorder the placeholder and the runtime value. The replacement is to
// embed a positional placeholder in `strings.xml` and pass the value as
// an argument to `stringResource(R.string.X, value)`.
type StringConcatForTranslationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection
// matches the call name `stringResource` without resolving the import
// site, so a same-named project symbol may produce a false match.
func (r *StringConcatForTranslationRule) Confidence() float64 { return 0.75 }

func (r *StringConcatForTranslationRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) != "additive_expression" {
		return
	}
	// Only emit on the outermost `+` chain so `a + b + c` reports once.
	if parent, ok := file.FlatParent(idx); ok {
		p := flatUnwrapParenExpr(file, parent)
		if file.FlatType(p) == "additive_expression" {
			return
		}
	}

	var operands []uint32
	if !collectStringConcatOperands(file, idx, &operands) {
		return
	}
	if len(operands) < 2 {
		return
	}

	hasStringResource := false
	hasNonLiteral := false
	for _, op := range operands {
		if isStringResourceCall(file, op) {
			hasStringResource = true
			continue
		}
		if !isStringConcatLiteralOperand(file, op) {
			hasNonLiteral = true
		}
	}

	if !hasStringResource || !hasNonLiteral {
		return
	}

	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Concatenating stringResource(...) with a non-literal forces English word order. Use a placeholder in strings.xml and pass arguments via stringResource(R.string.X, value).")
}

// collectStringConcatOperands flattens a `+`-only additive_expression
// chain into its leaf operands. Returns false if the expression mixes
// any non-`+` operator (e.g. subtraction), which means it is not a pure
// concatenation chain.
func collectStringConcatOperands(file *scanner.File, idx uint32, out *[]uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	if file.FlatType(idx) != "additive_expression" {
		*out = append(*out, idx)
		return true
	}
	var named []uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			named = append(named, child)
			continue
		}
		switch file.FlatNodeText(child) {
		case "+":
			// ok
		case "-":
			return false
		}
	}
	for _, op := range named {
		if !collectStringConcatOperands(file, op, out) {
			return false
		}
	}
	return true
}

// isStringResourceCall reports whether expr is a (parenthesized) call to
// `stringResource`. A multi-argument call already uses placeholders, so
// only the single-argument form is treated as a concat candidate.
func isStringResourceCall(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file.FlatType(expr) != "call_expression" {
		return false
	}
	if !flatCallExpressionNameEquals(file, expr, "stringResource") {
		return false
	}
	return stringResourceArgCount(file, expr) == 1
}

func stringResourceArgCount(file *scanner.File, call uint32) int {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return 0
	}
	n := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			n++
		}
	}
	return n
}

// isStringConcatLiteralOperand reports whether expr is a literal whose
// concatenation with a translated string is harmless (whitespace,
// punctuation, formatting). String templates with interpolation are
// treated as non-literal because they introduce dynamic content.
func isStringConcatLiteralOperand(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "integer_literal", "long_literal", "real_literal", "hex_literal",
		"bin_literal", "character_literal", "boolean_literal", "null_literal":
		return true
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		return !flatContainsStringInterpolation(file, expr)
	}
	return false
}
