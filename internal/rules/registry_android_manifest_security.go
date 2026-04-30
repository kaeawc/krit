package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidManifestSecurityRules() {

	// --- from android_manifest_security.go ---
	{
		r := &AllowBackupManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "AllowBackupManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "AllowBackup",
			Brief:      "Missing or true allowBackup attribute",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DebuggableManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DebuggableManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "HardcodedDebugMode",
			Brief:      "Hardcoded value of android:debuggable in manifest",
			Category:   ALCSecurity,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ExportedWithoutPermissionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ExportedWithoutPermission", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ExportedWithoutPermission",
			Brief:      "Exported component without required permission",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingExportedFlagRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingExportedFlag", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MissingExportedFlag",
			Brief:      "Component with intent-filter missing android:exported (API 31+)",
			Category:   ALCSecurity,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ExportedServiceManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ExportedServiceManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ExportedService",
			Brief:      "Exported service without required permission",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ExportedPreferenceActivityManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ExportedPreferenceActivityManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ExportedPreferenceActivity",
			Brief:      "Exported PreferenceActivity is vulnerable to fragment injection",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &CleartextTrafficRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "CleartextTraffic", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "CleartextTraffic",
			Brief:      "usesCleartextTraffic enabled",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &BackupRulesRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "BackupRules", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "BackupRules",
			Brief:      "Missing backup configuration",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &InsecureBaseConfigurationManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InsecureBaseConfigurationManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "InsecureBaseConfiguration",
			Brief:      "Missing networkSecurityConfig on API 28+",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnprotectedSMSBroadcastReceiverManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnprotectedSMSBroadcastReceiverManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UnprotectedSMSBroadcastReceiver",
			Brief:      "SMS receiver without permission protection",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnsafeProtectedBroadcastReceiverManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnsafeProtectedBroadcastReceiverManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UnsafeProtectedBroadcastReceiver",
			Brief:      "Exported receiver for protected broadcast without permission",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UseCheckPermissionManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UseCheckPermissionManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UseCheckPermission",
			Brief:      "Exported service with sensitive action but no permission",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ProtectedPermissionsManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ProtectedPermissionsManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "ProtectedPermissions",
			Brief:      "Using system app permission",
			Category:   ALCSecurity,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ServiceExportedManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ServiceExportedManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ServiceExported",
			Brief:      "Exported service does not require permission",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
