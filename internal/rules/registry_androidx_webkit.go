package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxWebkitRules registers the checks from androidx.webkit:webkit 1.18.0 and
// androidx.test.uiautomator:uiautomator 2.4.1. Lint's scope is JAVA_FILE_SCOPE, so test sources are not checked.
func registerAndroidxWebkitRules() {
	both := []scanner.Language{scanner.LangKotlin, scanner.LangJava}
	mainOnly := func(check func(*api.Context)) func(*api.Context) {
		return func(ctx *api.Context) {
			if !scanner.IsTestFile(ctx.File.Path) {
				check(ctx)
			}
		}
	}

	render := &MissingOnRenderProcessGoneRule{BaseRule: BaseRule{RuleName: "MissingOnRenderProcessGone",
		RuleSetName: "android-lint", Sev: "warning",
		Desc: "Detects WebViewClients that do not implement onRenderProcessGone, so a render-process crash takes the app down."}}
	registerAxlRule(render.BaseRule, render, []string{"class_declaration", "object_literal", "call_expression",
		"object_creation_expression"}, both, axlRuleOptions{}, mainOnly(render.check))

	allow := &CrossOriginIsolatedAllowlistRule{BaseRule: BaseRule{RuleName: "CrossOriginIsolatedAllowlist",
		RuleSetName: "android-lint", Sev: "warning",
		Desc: "Detects Profile.setCrossOriginIsolatedAllowlist, which enables cross-origin isolated APIs for the listed origins."}}
	registerAxlRule(allow.BaseRule, allow, []string{"assignment", "call_expression", "method_invocation"}, both,
		axlRuleOptions{}, mainOnly(allow.check))

	text := &DontUseAccessibilityNodeInfoGetTextRule{BaseRule: BaseRule{RuleName: "DontUseAccessibilityNodeInfoGetText",
		RuleSetName: "android-lint", Sev: "warning",
		Desc: "Detects AccessibilityNodeInfo.text in UiAutomator element predicates, where the CharSequence never equals a String."}}
	registerAxlRule(text.BaseRule, text, []string{"lambda_literal"}, []scanner.Language{scanner.LangKotlin},
		axlRuleOptions{}, text.check)
}
