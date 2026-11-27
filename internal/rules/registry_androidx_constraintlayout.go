package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxConstraintLayoutRules registers the checks from
// androidx.constraintlayout:constraintlayout-compose-android 1.1.2. Lint's scope is JAVA_FILE, so test sources are
// not checked.
func registerAndroidxConstraintLayoutRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	mainOnly := func(check func(*api.Context)) func(*api.Context) {
		return func(ctx *api.Context) {
			if !scanner.IsTestFile(projectPath(ctx, ctx.File.Path)) {
				check(ctx)
			}
		}
	}

	refs := &IncorrectReferencesDeclarationRule{BaseRule: BaseRule{RuleName: "IncorrectReferencesDeclaration",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects val (a, b) = createRefsFor(...) whose argument count differs from the destructured variables."}}
	registerAxlRule(refs.BaseRule, refs, []string{"call_expression"}, kotlin, axlRuleOptions{}, mainOnly(refs.check))

	match := &IncorrectMatchParentUsageRule{BaseRule: BaseRule{RuleName: "IncorrectMatchParentUsage",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects Dimension.matchParent next to custom constraints it overrides; prefer Dimension.percent(1f)."}}
	registerAxlRule(match.BaseRule, match, []string{"assignment"}, kotlin, axlRuleOptions{}, mainOnly(match.check))

	chain := &IncorrectChainMarginsUsageRule{BaseRule: BaseRule{RuleName: "IncorrectChainMarginsUsage",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects chain elements whose margins come from linkTo constraints, which chains ignore; use withChainParams."}}
	registerAxlRule(chain.BaseRule, chain, []string{"call_expression"}, kotlin, axlRuleOptions{}, mainOnly(chain.check))
}
