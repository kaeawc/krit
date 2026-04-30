package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerI18nStringTemplateRules() {

	// --- from i18n_string_template.go ---
	{
		r := &StringTemplateForTranslationRule{BaseRule: BaseRule{
			RuleName:    "StringTemplateForTranslation",
			RuleSetName: "i18n",
			Sev:         "info",
			Desc:        "Detects string templates that embed stringResource(...) alongside another dynamic interpolation, which forces English word order.",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes:      []string{"string_literal", "line_string_literal", "multi_line_string_literal"},
			Confidence:     r.Confidence(),
			Implementation: r,
			Check:          r.check,
		})
	}
}
