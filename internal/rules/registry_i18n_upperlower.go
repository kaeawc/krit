package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerI18nUpperLowerRules() {
	{
		r := &UpperLowerInvariantMisuseRule{BaseRule: BaseRule{
			RuleName:    "UpperLowerInvariantMisuse",
			RuleSetName: "i18n",
			Sev:         "warning",
			Desc:        "Detects Kotlin 1.5+ uppercase()/lowercase() called without an explicit Locale argument.",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
