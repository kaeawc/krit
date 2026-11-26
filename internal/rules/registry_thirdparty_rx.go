package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// slackLintRuleSet is the rule set the slack-lints ports register in (the MoshiUsage rules call it moshiRuleSet).
const slackLintRuleSet = "slack-lint"

// RxLintRule is one of slack-lints' Rx checks.
type RxLintRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a high base confidence: the factory or receiver library must be proven from imports.
func (r *RxLintRule) Confidence() float64 { return api.ConfidenceHigh }

func registerThirdPartyRxRules() {
	for _, rule := range []struct {
		id, brief string
		sev       api.Severity
		langs     []scanner.Language
		nodes     []string
		// callees narrows dispatch by callee name. The emit rules leave it unset: `rxObservable<T>(ctx) { }` parses
		// as a call whose callee is a call, which the lexical filter would skip.
		callees []string
		gate    func(*scanner.File) bool
		check   func(*api.Context)
	}{
		{rxSubscribeOnMain, "subscribeOn called with the main thread scheduler.", api.SeverityError,
			[]scanner.Language{scanner.LangKotlin, scanner.LangJava}, []string{"call_expression", "method_invocation"},
			[]string{"subscribeOn"}, rxSubscribeOnMainFileGate, checkSubscribeOnMain},
		{rxObservableDoesNotEmit, "RxObservable should call `send()` or `trySend()`", api.SeverityInfo,
			[]scanner.Language{scanner.LangKotlin}, []string{"call_expression"},
			nil, rxEmitFileGate, checkRxDoesNotEmit},
		{rxFlowableDoesNotEmit, "RxFlowable should call `send()` or `trySend()`", api.SeverityInfo,
			[]scanner.Language{scanner.LangKotlin}, []string{"call_expression"},
			nil, rxEmitFileGate, checkRxDoesNotEmit},
	} {
		r := &RxLintRule{BaseRule: BaseRule{RuleName: rule.id, RuleSetName: slackLintRuleSet, Sev: string(rule.sev), Desc: rule.brief}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: rule.sev,
			NodeTypes:          rule.nodes,
			LexicalCalleeNames: rule.callees,
			Languages:          rule.langs,
			FileGate:           rule.gate,
			Confidence:         r.Confidence(),
			Implementation:     r,
			Check:              rule.check,
			LintIssue:          true,
			DefaultActive:      false,
			OptInReason:        api.OptInReasonDomainSpecific,
			Maturity:           api.MaturityExperimental,
		})
	}
}
