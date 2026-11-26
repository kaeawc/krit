package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidxAnnotationExperimentalRules registers the declaration checks from
// androidx.annotation:annotation-experimental 1.6.0.
func registerAndroidxAnnotationExperimentalRules() {
	retention := &ExperimentalAnnotationRetentionRule{BaseRule: BaseRule{RuleName: "ExperimentalAnnotationRetention",
		RuleSetName: "android-lint", Sev: "error",
		Desc: "Detects opt-in marker annotations whose retention keeps them out of class files (Kotlin needs BINARY, Java CLASS)."}}
	registerAxlRule(retention.BaseRule, retention, []string{"class_declaration", "annotation_type_declaration"},
		[]scanner.Language{scanner.LangKotlin, scanner.LangJava}, axlRuleOptions{}, func(ctx *api.Context) {
			if !scanner.IsTestFile(ctx.File.Path) {
				retention.check(ctx)
			}
		})

	wrong := &WrongRequiresOptInRule{BaseRule: BaseRule{RuleName: "WrongRequiresOptIn", RuleSetName: "android-lint",
		Sev: "error", Desc: "Detects Kotlin opt-in markers declared with androidx.annotation.RequiresOptIn instead of kotlin.RequiresOptIn."}}
	registerAxlRule(wrong.BaseRule, wrong, []string{"class_declaration"}, []scanner.Language{scanner.LangKotlin},
		axlRuleOptions{}, func(ctx *api.Context) {
			if !scanner.IsTestFile(ctx.File.Path) {
				wrong.check(ctx)
			}
		})
}
