package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SlackDenyListRule is one of slack-lints' two deny-listed API checks.
type SlackDenyListRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a high base confidence: a finding needs the listed member's owner proven from imports, the
// class name, or the receiver's resolved or declared type.
func (r *SlackDenyListRule) Confidence() float64 { return api.ConfidenceHigh }

func registerThirdPartyDenyListRules() {
	for _, rule := range []struct {
		id, brief string
		nodes     []string
		gate      func(*scanner.File) bool
		check     func(*api.Context)
	}{
		{slackDenyListedAPI, "Deny-listed API",
			[]string{"call_expression", "method_invocation", "object_creation_expression", "navigation_expression", "field_access",
				"directly_assignable_expression", "import_header", "import_declaration"},
			denyListFileGate, checkDenyListedAPI},
		{slackDenyListedBlockingAPI, "Deny-listed API",
			[]string{"call_expression", "method_invocation"}, denyBlockingFileGate, checkDenyListedBlockingAPI},
	} {
		r := &SlackDenyListRule{BaseRule: BaseRule{RuleName: rule.id, RuleSetName: moshiRuleSet, Sev: string(api.SeverityError), Desc: rule.brief}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.SeverityError,
			NodeTypes:      rule.nodes,
			Languages:      []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Needs:          api.NeedsTypeInfo,
			FileGate:       rule.gate,
			Confidence:     r.Confidence(),
			Implementation: r,
			Check:          rule.check,
			LanguageSupport: map[string]api.LanguageSupport{JavaLanguageSupportKey: {Status: api.LanguageSupportSupported,
				Evidence: []string{"internal/rules/testdata/slack_lint/denylist"}}},
			LintIssue:     true,
			DefaultActive: false,
			OptInReason:   api.OptInReasonDomainSpecific,
			Maturity:      api.MaturityExperimental,
		})
	}
}
