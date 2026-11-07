package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MipmapLauncherRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MipmapLauncher", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MipmapIcons",
			Brief:      "Use Mipmap Launcher Icons",
			Category:   ALCUsability,
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
		r := &UniquePermissionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UniquePermission", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "UniquePermission",
			Brief:      "Permission names are not unique",
			Category:   ALCSecurity,
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
		r := &SystemPermissionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "SystemPermission", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "SystemPermission",
			Brief:      "Requesting a permission only granted to system apps",
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
		r := &ManifestTypoRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ManifestTypoManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "ManifestTypo",
			Brief:      "Typos in manifest element tags",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
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
		r := &MissingApplicationIconRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingApplicationIconManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MissingApplicationIcon",
			Brief:      "Missing android:icon on <application>",
			Category:   ALCUsability,
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
		r := &TargetNewerRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "TargetNewer", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "OldTargetApi",
			Brief:      "Target SDK version is too old",
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
		r := &IntentFilterExportRequiredRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "IntentFilterExportRequired", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "IntentFilterExportedReceiver",
			Brief:      "Component with intent-filter missing android:exported (API 31+)",
			Category:   ALCSecurity,
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MultipleUsesSdkManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MultipleUsesSdkManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MultipleUsesSdk",
			Brief:      "More than one <uses-sdk> element",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
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
		r := &ManifestOrderManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ManifestOrderManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ManifestOrder",
			Brief:      "<application> appears before <uses-permission> or <uses-sdk>",
			Category:   ALCCorrectness,
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
		r := &MissingVersionManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MissingVersionManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MissingVersion",
			Brief:      "Missing versionCode or versionName on <manifest>",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   2,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MockLocationManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MockLocationManifest", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "MockLocation",
			Brief:      "ACCESS_MOCK_LOCATION in non-debug manifest",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
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
		r := &UnpackedNativeCodeManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UnpackedNativeCodeManifest", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UnpackedNativeCode",
			Brief:      "Missing extractNativeLibs=false with native libraries",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &InvalidUsesFeatureRequiredManifestRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InvalidUsesFeatureRequiredManifest", RuleSetName: androidRuleSet, Sev: "error"},
			Brief:      "Invalid android:required value on <uses-feature>",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   5,
			Origin:     "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Aliases: []string{"InvalidUsesTagAttributeManifest"},
			Needs:   api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
