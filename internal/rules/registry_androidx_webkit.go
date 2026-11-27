package rules

import (
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// axlWebkitCoordinates and axlUiautomatorCoordinates are the artifacts whose AARs ship these checks. Lint only runs
// a library's checks when the project depends on it, so the rules require a declared dependency.
var (
	axlWebkitCoordinates      = []librarymodel.Coordinate{{Group: "androidx.webkit", Name: "webkit"}}
	axlUiautomatorCoordinates = []librarymodel.Coordinate{{Group: "androidx.test.uiautomator", Name: "uiautomator"}}
)

// registerAndroidxWebkitRules registers the checks from androidx.webkit:webkit 1.18.0 and
// androidx.test.uiautomator:uiautomator 2.4.1. Lint's scope is JAVA_FILE_SCOPE, so test sources are not checked.
func registerAndroidxWebkitRules() {
	both := []scanner.Language{scanner.LangKotlin, scanner.LangJava}
	gated := axlRuleOptions{NeedsLibraryFacts: true}
	mainOnly := func(coords []librarymodel.Coordinate, check func(*api.Context)) func(*api.Context) {
		return func(ctx *api.Context) {
			if !isProjectTestFile(ctx, ctx.File.Path) &&
				librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.HasAnyDependency(coords...) {
				check(ctx)
			}
		}
	}

	render := &MissingOnRenderProcessGoneRule{BaseRule: BaseRule{RuleName: "MissingOnRenderProcessGone",
		RuleSetName: "android-lint", Sev: "warning",
		Desc: "Detects WebViewClients that do not implement onRenderProcessGone, so a render-process crash takes the app down."}}
	registerAxlRule(render.BaseRule, render, []string{"class_declaration", "object_literal", "call_expression",
		"object_creation_expression"}, both, gated, mainOnly(axlWebkitCoordinates, render.check))

	allow := &CrossOriginIsolatedAllowlistRule{BaseRule: BaseRule{RuleName: "CrossOriginIsolatedAllowlist",
		RuleSetName: "android-lint", Sev: "warning",
		Desc: "Detects Profile.setCrossOriginIsolatedAllowlist, which enables cross-origin isolated APIs for the listed origins."}}
	registerAxlRule(allow.BaseRule, allow, []string{"assignment", "call_expression", "method_invocation"}, both,
		gated, mainOnly(axlWebkitCoordinates, allow.check))

	text := &DontUseAccessibilityNodeInfoGetTextRule{BaseRule: BaseRule{RuleName: "DontUseAccessibilityNodeInfoGetText",
		RuleSetName: "android-lint", Sev: "warning",
		Desc: "Detects AccessibilityNodeInfo.text in UiAutomator element predicates, where the CharSequence never equals a String."}}
	registerAxlRule(text.BaseRule, text, []string{"lambda_literal"}, []scanner.Language{scanner.LangKotlin},
		gated, mainOnly(axlUiautomatorCoordinates, text.check))
}
