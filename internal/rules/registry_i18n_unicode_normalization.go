package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			OptInReason: api.OptInReasonDomainSpecific,
		})
	}
}
