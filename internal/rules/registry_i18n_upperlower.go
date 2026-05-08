package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerI18nUpperLowerRules() {
	{
		r := &UpperLowerInvariantMisuseRule{BaseRule: BaseRule{
			RuleName:    "UpperLowerInvariantMisuse",
			RuleSetName: "i18n",
			Sev:         "warning",
			Desc:        "Detects Kotlin 1.5+ uppercase()/lowercase() called without an explicit Locale argument.",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: true,
		})
	}
}
