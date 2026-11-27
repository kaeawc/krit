package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SlackViewRule is one of slack-lints' View checks.
type SlackViewRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a high base confidence: the receiver's class and the enclosing class's supertypes are proven
// from the source resolver, declared types and imports before a finding is made.
func (r *SlackViewRule) Confidence() float64 { return api.ConfidenceHigh }

func registerThirdPartyViewCheckRules() {
	for _, rule := range []struct {
		id, brief string
		sev       api.Severity
		callee    string
		gate      func(*scanner.File) bool
		check     func(*api.Context)
	}{
		{slackDoNotCallViewToString, "Do not use `View.toString()`", api.SeverityError, "toString", viewToStringFileGate, checkDoNotCallViewToString},
		{slackInflationInItemDecoration, "Avoid inflating a view to display text", api.SeverityWarning, "inflate", viewInflateFileGate,
			checkInflationInItemDecoration},
	} {
		r := &SlackViewRule{BaseRule: BaseRule{RuleName: rule.id, RuleSetName: moshiRuleSet, Sev: string(rule.sev), Desc: rule.brief}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: rule.sev,
			NodeTypes:          []string{"call_expression", "method_invocation"},
			LexicalCalleeNames: []string{rule.callee},
			Languages:          []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Needs:              api.NeedsTypeInfo,
			FileGate:           rule.gate,
			Confidence:         r.Confidence(),
			Implementation:     r,
			Check:              rule.check,
			LanguageSupport: map[string]api.LanguageSupport{JavaLanguageSupportKey: {Status: api.LanguageSupportSupported,
				Evidence: []string{"internal/rules/testdata/slack_lint/view_checks"}}},
			LintIssue:     true,
			DefaultActive: false,
			OptInReason:   api.OptInReasonAndroidOnly,
			Maturity:      api.MaturityExperimental,
		})
	}
}
