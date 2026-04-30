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

	{
		r := &PluralsMissingZeroRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "PluralsMissingZero", RuleSetName: "i18n", Sev: "info"},
			IssueID:    "PluralsMissingZero",
			Brief:      "<plurals> in a CLDR zero-form locale is missing the zero quantity",
			Category:   ALCI18N,
			ALSeverity: ALSInformational,
			Priority:   3,
			Origin:     "Krit roadmap",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}

	{
		r := &StringResourcePlaceholderOrderRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{
				RuleName:    "StringResourcePlaceholderOrder",
				RuleSetName: "i18n",
				Sev:         "warning",
				Desc:        "Translation variants must keep positional format syntax (`%1$s`, `%2$s`) used by the default string.",
			},
			IssueID:    "StringResourcePlaceholderOrder",
			Brief:      "Variant string drops positional format syntax used by the default value",
			Category:   ALCI18N,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "Krit roadmap",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
