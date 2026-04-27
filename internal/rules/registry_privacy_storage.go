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
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"putString", "putInt", "putLong", "putFloat", "putBoolean", "putStringSet"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check:                  r.check,
		})
	}
	{
		r := &PlainFileWriteOfSensitiveRule{BaseRule: BaseRule{RuleName: "PlainFileWriteOfSensitive", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects plain-file writes to paths containing sensitive terms without using EncryptedFile."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"writeText", "writeBytes", "File"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check:                  r.check,
		})
	}
	{
		r := &LogOfSharedPreferenceReadRule{BaseRule: BaseRule{RuleName: "LogOfSharedPreferenceRead", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects logger calls that directly pass SharedPreferences values with sensitive keys."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"d", "i", "w", "e", "v", "wtf", "getString", "getInt", "getLong", "getFloat", "getBoolean", "getStringSet"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check:                  r.check,
		})
	}
}
