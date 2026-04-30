package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerI18nUnicodeNormalizationRules() {

	// --- from i18n_unicode_normalization.go ---
	{
		r := &UnicodeNormalizationMissingRule{BaseRule: BaseRule{
			RuleName:    "UnicodeNormalizationMissing",
			RuleSetName: "i18n",
			Sev:         "info",
			Desc:        "Detects contains() calls inside search/find functions that do not normalize operands; unicode-equivalent characters will not match.",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
