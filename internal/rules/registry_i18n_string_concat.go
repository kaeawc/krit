package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerI18nStringConcatRules() {

	// --- from i18n_string_concat.go ---
	{
		r := &StringConcatForTranslationRule{BaseRule: BaseRule{
			RuleName:    "StringConcatForTranslation",
			RuleSetName: "i18n",
			Sev:         "info",
			Desc:        "Detects `+` concatenation between stringResource(...) and a non-literal, which forces English word order.",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"additive_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			OptInReason:   api.OptInReasonDomainSpecific,
		})
	}
}
