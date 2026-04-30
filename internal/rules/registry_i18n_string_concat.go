package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"additive_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
