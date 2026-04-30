package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// StringTemplateForTranslationRule flags Kotlin string templates that
// embed a `stringResource(...)` interpolation alongside another dynamic
// interpolation. Like its concatenation sibling, the construct hard-codes
// English word order; translators cannot reorder the placeholder relative
// to the runtime value. The replacement is to embed positional
// placeholders in `strings.xml` and pass the values as arguments to
// `stringResource(R.string.X, value)`.
type StringTemplateForTranslationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection
// matches the call name `stringResource` without resolving the import
// site, so a same-named project symbol may produce a false match.
func (r *StringTemplateForTranslationRule) Confidence() float64 { return 0.75 }

func (r *StringTemplateForTranslationRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	switch file.FlatType(idx) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
	default:
		return
	}

	hasStringResource := false
	hasOtherDynamic := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "interpolated_identifier", "line_str_ref", "multi_line_str_ref":
			hasOtherDynamic = true
		case "interpolated_expression", "line_string_expression", "multi_line_string_expression":
			inner := flatStringTemplateInterpolationExpression(file, child)
			if inner != 0 && isStringResourceCall(file, inner) {
				hasStringResource = true
			} else {
				hasOtherDynamic = true
			}
		}
	}

	if !hasStringResource || !hasOtherDynamic {
		return
	}

	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"String template embeds stringResource(...) alongside another dynamic value, which forces English word order. Use a placeholder in strings.xml and pass arguments via stringResource(R.string.X, value).")
}
