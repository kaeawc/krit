package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxActivityComposeRules registers the checks from androidx.activity:activity-compose 1.13.0.
func registerAndroidxActivityComposeRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}

	launch := &LaunchDuringCompositionRule{BaseRule: BaseRule{RuleName: "LaunchDuringComposition", RuleSetName: "compose",
		Sev: "error", Desc: "Detects ManagedActivityResultLauncher.launch called during composition instead of inside a SideEffect."}}
	registerAxlRule(launch.BaseRule, launch, []string{"call_expression"}, kotlin, axlRuleOptions{}, launch.check)

	collect := &NoCollectCallFoundRule{BaseRule: BaseRule{RuleName: "NoCollectCallFound", RuleSetName: "compose",
		Sev: "error", Desc: "Detects PredictiveBackHandler onBack lambdas that never collect the back progress Flow."}}
	registerAxlRule(collect.BaseRule, collect, []string{"lambda_literal"}, kotlin, axlRuleOptions{}, collect.check)

	cast := &ContextCastToActivityRule{BaseRule: BaseRule{RuleName: "ContextCastToActivity", RuleSetName: "compose",
		Sev: "error", Desc: "Detects LocalContext.current cast to an Activity; use LocalActivity instead."}}
	registerAxlRule(cast.BaseRule, cast, []string{"as_expression"}, kotlin, axlRuleOptions{}, cast.check)
}
