package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SlackLintNodeRule is a single-node slack-lints check.
type SlackLintNodeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a high base confidence: every check resolves the names it needs through the file's imports
// and abstains when a same-file declaration could shadow them.
func (r *SlackLintNodeRule) Confidence() float64 { return api.ConfidenceHigh }

func registerThirdPartyKotlinCheckRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	for _, rule := range []struct {
		id, brief string
		sev       api.Severity
		langs     []scanner.Language
		nodes     []string
		callees   []string
		gate      func(*scanner.File) bool
		check     func(*api.Context)
	}{
		{slackAvoidUsingNotNullOperator, "Avoid using the !! operator in Kotlin", api.SeverityWarning, kotlin,
			[]string{"postfix_expression", "directly_assignable_expression"}, nil, notNullOperatorFileGate, checkAvoidUsingNotNullOperator},
		{slackExceptionMessage, "Please provide a string for the `lazyMessage` parameter", api.SeverityError, kotlin,
			[]string{"call_expression"}, []string{"check", "checkNotNull", "require", "requireNotNull"}, nil, checkExceptionMessage},
		{slackNullableConcurrentHashMap, "ConcurrentHashMap should not use nullable types", api.SeverityError,
			[]scanner.Language{scanner.LangKotlin, scanner.LangJava},
			[]string{"property_declaration", "class_parameter", "parameter", "call_expression",
				"local_variable_declaration", "field_declaration", "formal_parameter", "object_creation_expression"},
			nil, concurrentHashMapFileGate, checkNullableConcurrentHashMap},
		{slackGuavaPreconditionsUsedInKotlin, "Kotlin precondition checks should use the Kotlin standard library checks", api.SeverityError,
			kotlin, []string{"call_expression"}, nil, guavaPreconditionsFileGate, checkGuavaPreconditionsUsedInKotlin},
		{slackParcelizeFunctionProperty, "Function type properties are not parcelable", api.SeverityError, kotlin,
			[]string{"class_declaration"}, nil, parcelizeFileGate, checkParcelizeFunctionProperty},
	} {
		r := &SlackLintNodeRule{BaseRule: BaseRule{RuleName: rule.id, RuleSetName: moshiRuleSet, Sev: string(rule.sev), Desc: rule.brief}}
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
