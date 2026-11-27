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
			if !isProjectTestFile(ctx, ctx.File.Path) {
				retention.check(ctx)
			}
		})

	wrong := &WrongRequiresOptInRule{BaseRule: BaseRule{RuleName: "WrongRequiresOptIn", RuleSetName: "android-lint",
		Sev: "error", Desc: "Detects Kotlin opt-in markers declared with androidx.annotation.RequiresOptIn instead of kotlin.RequiresOptIn."}}
	registerAxlRule(wrong.BaseRule, wrong, []string{"class_declaration"}, []scanner.Language{scanner.LangKotlin},
		axlRuleOptions{}, func(ctx *api.Context) {
			if !isProjectTestFile(ctx, ctx.File.Path) {
				wrong.check(ctx)
			}
		})

	optInError := &UnsafeOptInUsageErrorRule{BaseRule: BaseRule{RuleName: "UnsafeOptInUsageError", RuleSetName: "android-lint",
		Sev: "error", Desc: "Detects uses of APIs marked with an error-level opt-in marker declared in the project, without opting in."}}
	api.Register(&api.Rule{
		ID: optInError.RuleName, Category: optInError.RuleSetName, Description: optInError.Desc,
		Sev: api.Severity(optInError.Sev), Needs: api.NeedsParsedFiles,
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium,
		Implementation: optInError, LintIssue: true,
		Check: func(ctx *api.Context) { axlOptInCheck(ctx, optInError.BaseRule, false) },
	})

	optInWarning := &UnsafeOptInUsageWarningRule{BaseRule: BaseRule{RuleName: "UnsafeOptInUsageWarning",
		RuleSetName: "android-lint", Sev: "warning",
		Desc: "Detects uses of APIs marked with a warning-level opt-in marker declared in the project, without opting in."}}
	api.Register(&api.Rule{
		ID: optInWarning.RuleName, Category: optInWarning.RuleSetName, Description: optInWarning.Desc,
		Sev: api.Severity(optInWarning.Sev), Needs: api.NeedsParsedFiles,
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium,
		Implementation: optInWarning, LintIssue: true,
		Check: func(ctx *api.Context) { axlOptInCheck(ctx, optInWarning.BaseRule, true) },
	})
}
