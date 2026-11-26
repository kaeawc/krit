package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxComposeAnimationRules registers the checks from androidx.compose.animation:animation-android and
// animation-core-android 1.13.0.
func registerAndroidxComposeAnimationRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	lambda := []string{"lambda_literal"}

	crossfade := &UnusedCrossfadeTargetStateParameterRule{BaseRule: BaseRule{RuleName: "UnusedCrossfadeTargetStateParameter",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects Crossfade content lambdas that ignore the target state parameter, so the same content fades into itself."}}
	registerAxlRule(crossfade.BaseRule, crossfade, lambda, kotlin, axlRuleOptions{}, crossfade.check)

	content := &UnusedContentLambdaTargetStateParameterRule{BaseRule: BaseRule{RuleName: "UnusedContentLambdaTargetStateParameter",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects AnimatedContent content lambdas that ignore the target state parameter."}}
	registerAxlRule(content.BaseRule, content, lambda, kotlin, axlRuleOptions{}, content.check)

	key := &UnusedTargetStateInContentKeyLambdaRule{BaseRule: BaseRule{RuleName: "UnusedTargetStateInContentKeyLambda",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects AnimatedContent contentKey lambdas that ignore the target state parameter."}}
	registerAxlRule(key.BaseRule, key, lambda, kotlin, axlRuleOptions{}, key.check)

	transition := &UnusedTransitionTargetStateParameterRule{BaseRule: BaseRule{RuleName: "UnusedTransitionTargetStateParameter",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects Transition.animate* lambdas that compute their value without the target state parameter."}}
	registerAxlRule(transition.BaseRule, transition, lambda, kotlin, axlRuleOptions{}, transition.check)

	shared := &UnusedSharedTransitionModifierParameterRule{BaseRule: BaseRule{RuleName: "UnusedSharedTransitionModifierParameter",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects SharedTransitionScope content lambdas that ignore the Modifier parameter meant for the top-most composable."}}
	registerAxlRule(shared.BaseRule, shared, lambda, kotlin, axlRuleOptions{}, shared.check)

	constKey := &ConstantContentStateKeyInItemsCallRule{BaseRule: BaseRule{RuleName: "ConstantContentStateKeyInItemsCall",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects shared elements in LazyList items keyed by a constant rememberSharedContentState key, so all items share one key."}}
	registerAxlRule(constKey.BaseRule, constKey, []string{"call_expression"}, kotlin, axlRuleOptions{}, constKey.check)

	debug := &DisallowLookaheadAnimationVisualDebugRule{BaseRule: BaseRule{RuleName: "DisallowLookaheadAnimationVisualDebug",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects LookaheadAnimationVisualDebugging debugging wrappers left in production code."}}
	registerAxlRule(debug.BaseRule, debug, []string{"call_expression"}, kotlin, axlRuleOptions{}, debug.check)

	arc := &ArcAnimationSpecTypeIssueRule{BaseRule: BaseRule{RuleName: "ArcAnimationSpecTypeIssue", RuleSetName: "compose",
		Sev: "info", Desc: "Detects ArcAnimationSpec used for values other than the 2D offsets it is designed for."}}
	registerAxlRule(arc.BaseRule, arc, []string{"call_expression"}, kotlin, axlRuleOptions{}, arc.check)
}
