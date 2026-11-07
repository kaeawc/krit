package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerAndroidManifestFeaturesRules() {

	// --- from android_manifest_features.go ---
	{
		r := &RtlEnabledManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RtlEnabledManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RtlEnabled",
			Brief:      "Unset supportsRtl although resources use RTL attributes, or supportsRtl=true while targeting below API 17",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &GoogleAppIndexingDeepLinkErrorManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GoogleAppIndexingDeepLinkErrorManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "GoogleAppIndexingDeepLinkError",
			Brief:      "Web deep link intent filter missing action, categories, host or valid path",
			Category:   ALCUsability,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Aliases: []string{"AppIndexingErrorManifest", "AppIndexingError"},
			Needs:   api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &GoogleAppIndexingWarningManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GoogleAppIndexingWarningManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "GoogleAppIndexingWarning",
			Brief:      "No activity with deep link support",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Aliases: []string{"AppIndexingWarningManifest", "AppIndexingWarning"},
			Needs:   api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingLeanbackLauncherManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingLeanbackLauncherManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MissingLeanbackLauncher",
			Brief:      "Leanback feature without LEANBACK_LAUNCHER activity",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingLeanbackSupportManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingLeanbackSupportManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MissingLeanbackSupport",
			Brief:      "Missing Leanback Support",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &PermissionImpliesUnsupportedHardwareManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "PermissionImpliesUnsupportedHardwareManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "PermissionImpliesUnsupportedHardware",
			Brief:      "Permission Implies Unsupported Hardware",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnsupportedChromeOsHardwareManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnsupportedChromeOsHardwareManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "UnsupportedChromeOsHardware",
			Brief:      "Hardware feature unsupported on Chrome OS not marked optional",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DeviceAdminManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DeviceAdminManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "DeviceAdmin",
			Brief:      "Malformed Device Admin",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &FullBackupContentManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "FullBackupContentManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "FullBackupContent",
			Brief:      "Invalid fullBackupContent or dataExtractionRules",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingRegisteredManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingRegisteredManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MissingRegistered",
			Brief:      "Component name in the manifest is empty or malformed",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
