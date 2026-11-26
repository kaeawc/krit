package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SlackAndroidRule is one of slack-lints' Android source checks.
type SlackAndroidRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a high base confidence: the receiver (View, Resources) or the constant's owner (Spanned) is
// proven from resolved types or imports before a finding is made.
func (r *SlackAndroidRule) Confidence() float64 { return api.ConfidenceHigh }

func registerThirdPartyAndroidUIRules() {
	both := []scanner.Language{scanner.LangKotlin, scanner.LangJava}
	javaSupport := func(evidence string) map[string]api.LanguageSupport {
		return map[string]api.LanguageSupport{JavaLanguageSupportKey: {Status: api.LanguageSupportSupported, Evidence: []string{evidence}}}
	}
	for _, rule := range []struct {
		id, brief string
		sev       api.Severity
		nodes     []string
		callees   []string
		needs     api.Capabilities
		gate      func(*scanner.File) bool
		check     func(*api.Context)
	}{
		{slackCastingViewContextToActivity, "Unsafe cast of `Context` to `Activity`", api.SeverityError,
			[]string{"as_expression", "cast_expression"}, nil, api.NeedsTypeInfo, viewContextFileGate, checkCastingViewContextToActivity},
		{slackArgInFormattedQuantityStringRes, "Count value in formatted string resource.", api.SeverityWarning,
			[]string{"call_expression", "method_invocation"}, []string{"getQuantityString"}, api.NeedsTypeInfo,
			quantityStringFileGate, checkArgInFormattedQuantityStringRes},
		{slackSpanMarkPointMissingMask, "Check that Span flags use the bitwise mask SPAN_POINT_MARK_MASK when being compared to.",
			api.SeverityError, []string{"equality_expression", "binary_expression"}, nil, 0, spanMarkPointFileGate, checkSpanMarkPointMissingMask},
	} {
		r := &SlackAndroidRule{BaseRule: BaseRule{RuleName: rule.id, RuleSetName: moshiRuleSet, Sev: string(rule.sev), Desc: rule.brief}}
		reg := &api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: rule.sev,
			NodeTypes:          rule.nodes,
			LexicalCalleeNames: rule.callees,
			Languages:          both,
			Needs:              rule.needs,
			FileGate:           rule.gate,
			Confidence:         r.Confidence(),
			Implementation:     r,
			Check:              rule.check,
			LintIssue:          true,
			DefaultActive:      false,
			OptInReason:        api.OptInReasonAndroidOnly,
			Maturity:           api.MaturityExperimental,
		}
		if rule.needs != 0 {
			reg.LanguageSupport = javaSupport("internal/rules/testdata/slack_lint/android_ui")
		}
		api.Register(reg)
	}
}
