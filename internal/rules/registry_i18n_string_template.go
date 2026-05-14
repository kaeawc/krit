package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:      []string{"string_literal", "line_string_literal", "multi_line_string_literal"},
			Confidence:     r.Confidence(),
			Implementation: r,
			Check:          r.check,
			DefaultActive:  false,
			OptInReason: api.OptInReasonDomainSpecific,
		})
	}
}
