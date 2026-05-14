package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerI18nTextDirectionRules() {

	// --- from i18n_text_direction.go ---
	{
		r := &TextDirectionLiteralInStringRule{BaseRule: BaseRule{
			RuleName:    "TextDirectionLiteralInString",
			RuleSetName: "i18n",
			Sev:         "info",
			Desc:        "Detects string literals that embed Unicode BIDI control characters instead of using a directional formatter.",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			OptInReason:   api.OptInReasonDomainSpecific,
		})
	}
}
