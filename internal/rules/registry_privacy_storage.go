package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPrivacyStorageRules() {

	// --- from privacy_storage.go ---
	{
		r := &SharedPreferencesForSensitiveKeyRule{BaseRule: BaseRule{RuleName: "SharedPreferencesForSensitiveKey", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects SharedPreferences put calls with key names matching sensitive patterns like token, password, or secret."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: api.NeedsResolver | api.NeedsOracleCallTargets | api.NeedsOracleExprType,
			Oracle: &api.OracleFilter{Identifiers: []string{"putString", "putInt", "putStringSet"}}, Confidence: api.ConfidenceMedium, Implementation: r,
			TypeInfo: api.TypeInfoHint{PreferBackend: api.PreferOracle, Required: true},
			Check:    r.check,
		})
	}
	{
		r := &PlainFileWriteOfSensitiveRule{BaseRule: BaseRule{RuleName: "PlainFileWriteOfSensitive", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects plain-file writes to paths containing sensitive terms without using EncryptedFile."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: api.NeedsResolver | api.NeedsOracleCallTargets | api.NeedsOracleExprType,
			Oracle: &api.OracleFilter{Identifiers: []string{"writeText", "writeBytes"}}, Confidence: api.ConfidenceMedium, Implementation: r,
			TypeInfo: api.TypeInfoHint{PreferBackend: api.PreferOracle, Required: true},
			Check:    r.check,
		})
	}
	{
		r := &LogOfSharedPreferenceReadRule{BaseRule: BaseRule{RuleName: "LogOfSharedPreferenceRead", RuleSetName: privacyRuleSet, Sev: "warning", Desc: "Detects logger calls that directly pass SharedPreferences values with sensitive keys."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: api.NeedsResolver | api.NeedsOracleCallTargets | api.NeedsOracleExprType,
			Oracle: &api.OracleFilter{Identifiers: []string{"getString", "getInt", "getStringSet"}}, Confidence: api.ConfidenceMedium, Implementation: r,
			TypeInfo: api.TypeInfoHint{PreferBackend: api.PreferOracle, Required: true},
			Check:    r.check,
		})
	}
}
