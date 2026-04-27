package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidManifestStructureRules() {

	// --- from android_manifest_structure.go ---
	{
		r := &DuplicateActivityManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DuplicateActivityManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "DuplicateActivity",
			Brief:      "Activity registered more than once",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &WrongManifestParentManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "WrongManifestParentManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "WrongManifestParent",
			Brief:      "Element declared under wrong parent in manifest",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GradleOverridesManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradleOverridesManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "GradleOverrides",
			Brief:      "SDK version in manifest overridden by Gradle",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &UsesSdkManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UsesSdkManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UsesMinSdkAttributes",
			Brief:      "Missing <uses-sdk> element in manifest",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   9,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MipmapLauncherRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MipmapLauncher", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MipmapIcons",
			Brief:      "Launcher icon should use @mipmap/ not @drawable/",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &UniquePermissionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UniquePermission", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "UniquePermission",
			Brief:      "Custom permission collides with system permission",
			Category:   ALCSecurity,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &SystemPermissionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "SystemPermission", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "SystemPermission",
			Brief:      "Requesting dangerous system permission",
			Category:   ALCSecurity,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ManifestTypoRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ManifestTypoManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "ManifestTypo",
			Brief:      "Typos in manifest element tags",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MissingApplicationIconRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingApplicationIconManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MissingApplicationIcon",
			Brief:      "Missing android:icon on <application>",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &TargetNewerRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "TargetNewer", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "OldTargetApi",
			Brief:      "Target SDK version is too old",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &IntentFilterExportRequiredRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "IntentFilterExportRequired", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "IntentFilterExportRequired",
			Brief:      "Component with intent-filter missing android:exported (API 31+)",
			Category:   ALCSecurity,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &DuplicateUsesFeatureManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DuplicateUsesFeatureManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "DuplicateUsesFeature",
			Brief:      "Duplicate <uses-feature> declaration",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MultipleUsesSdkManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MultipleUsesSdkManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MultipleUsesSdk",
			Brief:      "More than one <uses-sdk> element",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ManifestOrderManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ManifestOrderManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ManifestOrder",
			Brief:      "<application> appears before <uses-permission> or <uses-sdk>",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MissingVersionManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingVersionManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MissingVersion",
			Brief:      "Missing versionCode or versionName on <manifest>",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   2,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MockLocationManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MockLocationManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MockLocation",
			Brief:      "ACCESS_MOCK_LOCATION in non-debug manifest",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &UnpackedNativeCodeManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnpackedNativeCodeManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UnpackedNativeCode",
			Brief:      "Missing extractNativeLibs=false with native libraries",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &InvalidUsesTagAttributeManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InvalidUsesTagAttributeManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "InvalidUsesTagAttribute",
			Brief:      "Invalid android:required value on <uses-feature>",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
