package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerThirdPartyJavaOnlyRules() {
	r := &SlackAnnotationRule{BaseRule: BaseRule{RuleName: slackJavaOnlyDetector, RuleSetName: moshiRuleSet,
		Sev: string(api.SeverityError), Desc: "Using @JavaOnly elements in Kotlin code."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.SeverityError,
		Needs: api.NeedsCrossFile | api.NeedsResolver,
		// The detector checks Kotlin sources only; Java declarations are indexed for them.
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		LanguageSupport: map[string]api.LanguageSupport{JavaLanguageSupportKey: {
			Status:   api.LanguageSupportPartial,
			Reason:   "As in Lint, only Kotlin call sites and declarations are checked; Java declarations are read for them.",
			Evidence: []string{"internal/rules/testdata/slack_lint_crossfile/javaonly"},
		}},
		Confidence: r.Confidence(), Implementation: r, Check: emitJavaOnlyFindings,
		LintIssue: true, DefaultActive: false, OptInReason: api.OptInReasonDomainSpecific, Maturity: api.MaturityExperimental,
	})
}
