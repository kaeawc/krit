package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: true,
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: true,
		})
	}
}
