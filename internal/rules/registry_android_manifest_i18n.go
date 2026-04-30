package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
