package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerI18nStringContainsHTMLWithoutCDATARules() {

	// --- from i18n_string_contains_html_without_cdata.go ---
	{
		r := &StringContainsHTMLWithoutCDATARule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringContainsHtmlWithoutCDATA", RuleSetName: "i18n", Sev: "info"},
			IssueID:    "StringContainsHtmlWithoutCDATA",
			Brief:      "<string> value contains literal HTML markup not wrapped in <![CDATA[...]]> or entity-escaped.",
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
}
