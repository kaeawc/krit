package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidGradleRules() {

	// --- from android_gradle.go ---
	{
		r := &GradlePluginCompatibilityRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradlePluginCompatibility", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "GradleCompatibility",
			Brief:      "AGP version incompatible with Gradle version",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StringIntegerRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringInteger", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "StringInteger",
			Brief:      "String value where integer expected in Gradle DSL",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &RemoteVersionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RemoteVersion", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RemoteVersion",
			Brief:      "Non-deterministic dependency version (+ or latest)",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &DynamicVersionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DynamicVersion", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "DynamicVersion",
			Brief:      "Dynamic dependency version with partial wildcard",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GradleOldTargetApiRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "OldTargetApi", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "OldTargetApi",
			Brief:      "targetSdkVersion below recommended minimum",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}, Threshold: 33}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &DeprecatedDependencyRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DeprecatedDependency", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "DeprecatedDependency",
			Brief:      "Deprecated library dependency",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MavenLocalRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MavenLocal", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MavenLocal",
			Brief:      "mavenLocal() causes unreproducible builds",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MinSdkTooLowRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MinSdkTooLow", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MinSdkTooLow",
			Brief:      "minSdkVersion below recommended minimum",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}, Threshold: 21}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GradleDeprecatedRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradleDeprecated", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "GradleDeprecated",
			Brief:      "Deprecated Gradle construct",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GradleGetterRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradleGetter", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "GradleGetter",
			Brief:      "Gradle implicit getter call",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GradlePathRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradlePath", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "GradlePath",
			Brief:      "Gradle path issues",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GradleOverridesRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradleOverrides", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "GradleOverrides",
			Brief:      "Value overridden by Gradle build script",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GradleIdeErrorRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradleIdeError", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "GradleIdeError",
			Brief:      "Gradle IDE Support Issues",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &AndroidGradlePluginVersionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "AndroidGradlePluginVersion", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "AndroidGradlePluginVersion",
			Brief:      "AGP version too old",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NewerVersionAvailableRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "NewerVersionAvailable", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "NewerVersionAvailable",
			Brief:      "Newer library version available",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StringIntegerRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StringShouldBeInt", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "StringShouldBeInt",
			Brief:      "String value where integer expected in Gradle DSL",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &GradlePluginCompatibilityRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradleCompatible", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "GradleCompatible",
			Brief:      "Incompatible Gradle versions",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NewerVersionAvailableRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradleDependency", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "GradleDependency",
			Brief:      "Obsolete Gradle dependency",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &DynamicVersionRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "GradleDynamicVersion", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "GradleDynamicVersion",
			Brief:      "Gradle dynamic version",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
