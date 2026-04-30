package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidManifestFeaturesRules() {

	// --- from android_manifest_features.go ---
	{
		r := &RtlEnabledManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RtlEnabledManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RtlEnabled",
			Brief:      "Missing supportsRtl=true on <application>",
			Category:   ALCUsability,
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
		r := &RtlCompatManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RtlCompatManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RtlCompat",
			Brief:      "Missing supportsRtl with targetSdkVersion >= 17",
			Category:   ALCI18N,
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
		r := &AppIndexingErrorManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "AppIndexingErrorManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "AppIndexingError",
			Brief:      "VIEW intent filter missing http/https data scheme",
			Category:   ALCUsability,
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
		r := &AppIndexingWarningManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "AppIndexingWarningManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "AppIndexingWarning",
			Brief:      "Browsable intent filter missing VIEW action",
			Category:   ALCUsability,
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
		r := &GoogleAppIndexingDeepLinkErrorManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GoogleAppIndexingDeepLinkErrorManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "GoogleAppIndexingDeepLinkError",
			Brief:      "Deep link data element with scheme but no host",
			Category:   ALCUsability,
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
		r := &GoogleAppIndexingWarningManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GoogleAppIndexingWarningManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "GoogleAppIndexingWarning",
			Brief:      "No activity with deep link support",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingLeanbackSupportManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingLeanbackSupportManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MissingLeanbackSupport",
			Brief:      "Leanback feature without touchscreen opt-out",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
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
		r := &PermissionImpliesUnsupportedHardwareManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "PermissionImpliesUnsupportedHardwareManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "PermissionImpliesUnsupportedHardware",
			Brief:      "Permission implies hardware feature not declared optional",
			Category:   ALCCorrectness,
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
		r := &UnsupportedChromeOsHardwareManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnsupportedChromeOsHardwareManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UnsupportedChromeOsHardware",
			Brief:      "Hardware feature unsupported on Chrome OS not marked optional",
			Category:   ALCCorrectness,
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
		r := &DeviceAdminManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DeviceAdminManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "DeviceAdmin",
			Brief:      "Malformed Device Admin",
			Category:   ALCCorrectness,
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
		r := &FullBackupContentManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "FullBackupContentManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "FullBackupContent",
			Brief:      "Invalid fullBackupContent or dataExtractionRules",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
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
		r := &MissingRegisteredManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingRegisteredManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MissingRegistered",
			Brief:      "Missing registered class",
			Category:   ALCCorrectness,
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
}
