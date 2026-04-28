package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerPrivacyStorageRules() {

	// --- from privacy_storage.go ---
	{
		r := &SharedPreferencesForSensitiveKeyRule{BaseRule: BaseRule{RuleName: "SharedPreferencesForSensitiveKey", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects SharedPreferences put calls with key names matching sensitive patterns like token, password, or secret."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsResolver, Confidence: 0.75, OriginalV1: r,
			TypeInfo: v2.TypeInfoHint{PreferBackend: v2.PreferResolver, Required: true},
			Check:    r.check,
		})
	}
	{
		r := &PlainFileWriteOfSensitiveRule{BaseRule: BaseRule{RuleName: "PlainFileWriteOfSensitive", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects plain-file writes to paths containing sensitive terms without using EncryptedFile."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsResolver, Confidence: 0.75, OriginalV1: r,
			TypeInfo: v2.TypeInfoHint{PreferBackend: v2.PreferResolver, Required: true},
			Check:    r.check,
		})
	}
	{
		r := &LogOfSharedPreferenceReadRule{BaseRule: BaseRule{RuleName: "LogOfSharedPreferenceRead", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects logger calls that directly pass SharedPreferences values with sensitive keys."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsResolver, Confidence: 0.75, OriginalV1: r,
			TypeInfo: v2.TypeInfoHint{PreferBackend: v2.PreferResolver, Required: true},
			Check:    r.check,
		})
	}
}
