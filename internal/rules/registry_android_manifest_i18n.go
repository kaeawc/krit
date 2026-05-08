package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerAndroidManifestI18nRules() {

	// --- from android_manifest_i18n.go ---
	{
		r := &LocaleConfigMissingRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "LocaleConfigMissing", RuleSetName: androidRuleSet, Sev: "info"},
			IssueID:    "LocaleConfigMissing",
			Brief:      "android:localeConfig points at a missing XML resource",
			Category:   ALCI18N,
			ALSeverity: ALSInformational,
			Priority:   3,
			Origin:     "Krit roadmap",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
