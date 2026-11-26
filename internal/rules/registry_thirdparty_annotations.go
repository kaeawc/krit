package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SlackAnnotationRule is one of the slack-lints checks driven by an annotation.
type SlackAnnotationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a high base confidence: a call is matched to an annotated declaration only through its import,
// package, class or a resolved receiver type.
func (r *SlackAnnotationRule) Confidence() float64 { return api.ConfidenceHigh }

func registerThirdPartyAnnotationRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	tp := &SlackAnnotationRule{BaseRule: BaseRule{RuleName: slackTestParameterSiteTarget, RuleSetName: moshiRuleSet,
		Sev: string(api.SeverityError), Desc: "`TestParameter` annotation has the wrong site target"}}
	api.Register(&api.Rule{
		ID: tp.RuleName, Category: tp.RuleSetName, Description: tp.Desc, Sev: api.SeverityError,
		NodeTypes: []string{"class_parameter"}, Languages: kotlin, FileGate: testParameterFileGate,
		Confidence: tp.Confidence(), Implementation: tp, Check: checkTestParameterSiteTarget,
		LintIssue: true, DefaultActive: false, OptInReason: api.OptInReasonDomainSpecific, Maturity: api.MaturityExperimental,
	})
	for _, rule := range []struct{ id, brief string }{
		{slackMustUseNamedParams, "Calls to @MustUseNamedParams-annotated methods must name all parameters."},
		{slackRestrictCallsTo, "Methods annotated with @RestrictedCallsTo should only be called from the specified scope."},
	} {
		r := &SlackAnnotationRule{BaseRule: BaseRule{RuleName: rule.id, RuleSetName: moshiRuleSet, Sev: string(api.SeverityError), Desc: rule.brief}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.SeverityError,
			Needs:     api.NeedsCrossFile | api.NeedsResolver,
			Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			LanguageSupport: map[string]api.LanguageSupport{JavaLanguageSupportKey: {
				Status:   api.LanguageSupportPartial,
				Evidence: []string{"internal/rules/testdata/slack_lint/annotations"},
			}},
			Confidence: r.Confidence(), Implementation: r, Check: emitSlackAnnotatedCalls,
			LintIssue: true, DefaultActive: false, OptInReason: api.OptInReasonDomainSpecific, Maturity: api.MaturityExperimental,
		})
	}
}
