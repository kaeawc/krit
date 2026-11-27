package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxComposeFoundationRules registers the checks from androidx.compose.foundation:foundation-android
// 1.13.0.
func registerAndroidxComposeFoundationRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}

	offset := &UseOfNonLambdaOffsetOverloadRule{BaseRule: BaseRule{RuleName: "UseOfNonLambdaOffsetOverload",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects Modifier.offset(x, y) with State-backed arguments, where the lambda overload offset { } avoids recomposition."}}
	registerAxlRule(offset.BaseRule, offset, []string{"call_expression"}, kotlin, axlRuleOptions{}, offset.check)

	box := &UnusedBoxWithConstraintsScopeRule{BaseRule: BaseRule{RuleName: "UnusedBoxWithConstraintsScope",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects BoxWithConstraints whose content never uses the constraints, so a Box would avoid subcomposition."}}
	registerAxlRule(box.BaseRule, box, []string{"lambda_literal"}, kotlin, axlRuleOptions{}, box.check)

	appendRule := &TextFieldBufferInternalAppendRule{BaseRule: BaseRule{RuleName: "TextFieldBufferInternalAppend",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects TextFieldBuffer.append in androidx.compose.foundation code, which drops hardware-source tracking."}}
	registerAxlRule(appendRule.BaseRule, appendRule, []string{"call_expression"}, kotlin, axlRuleOptions{},
		appendRule.check)
}
