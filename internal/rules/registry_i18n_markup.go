package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerI18nMarkupRules() {

	// --- from i18n_markup.go ---
	{
		r := &TranslatableMarkupMismatchRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "TranslatableMarkupMismatch", RuleSetName: "i18n", Sev: "warning"},
			IssueID:    "TranslatableMarkupMismatch",
			Brief:      "<string> markup style differs across locale variants (HTML vs Markdown vs plain text).",
			Category:   ALCI18N,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "Krit roadmap",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
