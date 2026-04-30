package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerI18nPluralsRules() {

	// --- from i18n_plurals.go ---
	{
		r := &PluralsBuiltWithIfElseRule{BaseRule: BaseRule{
			RuleName:    "PluralsBuiltWithIfElse",
			RuleSetName: "i18n",
			Sev:         "warning",
			Desc:        "Detects manual pluralization built with if/else over count == 1 instead of getQuantityString / pluralStringResource.",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
