package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerLicensingRules() {

	// --- from licensing.go ---
	{
		r := &CopyrightYearOutdatedRule{
			BaseRule:         BaseRule{RuleName: "CopyrightYearOutdated", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects stale copyright years in file header comments."},
			RecentYearCutoff: recentCopyrightYearCutoff,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingSpdxIdentifierRule{
			BaseRule:       BaseRule{RuleName: "MissingSpdxIdentifier", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects file header comments that are missing a SPDX license identifier."},
			RequiredPrefix: spdxIdentifierPrefix,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &LgplStaticLinkingInApkRule{
			BaseRule: BaseRule{RuleName: "LgplStaticLinkingInApk", RuleSetName: licensingRuleSet, Sev: "warning", Desc: "Detects Android application modules that statically link known-LGPL dependencies into the APK."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &OssLicensesNotIncludedInAndroidRule{
			BaseRule: BaseRule{RuleName: "OssLicensesNotIncludedInAndroid", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects Android app modules with implementation dependencies but no attribution surface (oss-licenses-plugin or LICENSE file)."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DependencyLicenseIncompatibleRule{
			BaseRule: BaseRule{RuleName: "DependencyLicenseIncompatible", RuleSetName: licensingRuleSet, Sev: "warning", Desc: "Detects external dependencies whose license is incompatible with the project's declared license."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &OptInMarkerNotRecognisedRule{
			BaseRule: BaseRule{RuleName: "OptInMarkerNotRecognised", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects @OptIn marker classes not in the embedded well-known markers list."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"annotation"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DependencyLicenseUnknownRule{
			BaseRule: BaseRule{RuleName: "DependencyLicenseUnknown", RuleSetName: licensingRuleSet, Sev: "info", Desc: "Detects external dependencies not present in the embedded license registry."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
