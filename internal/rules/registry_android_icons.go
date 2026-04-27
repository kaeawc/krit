package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidIconsRules() {

	// --- from android_icons.go ---
	{
		r := &IconDensitiesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "IconDensities", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "IconDensities", Brief: "Missing density variants for icon",
			Category: ALCIcons, ALSeverity: ALSWarning, Priority: 4,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
	{
		r := &IconDipSizeRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "IconDipSize", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "IconDipSize", Brief: "Icon dimensions don't match expected DPI ratios",
			Category: ALCIcons, ALSeverity: ALSWarning, Priority: 4,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
	{
		r := &IconDuplicatesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "IconDuplicates", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "IconDuplicates", Brief: "Same image across densities without scaling",
			Category: ALCIcons, ALSeverity: ALSWarning, Priority: 3,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
	{
		r := &GifUsageRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "GifUsage", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "GifUsage", Brief: "GIF file in resources",
			Category: ALCIcons, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
	{
		r := &ConvertToWebpRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ConvertToWebp", RuleSetName: androidRuleSet, Sev: "informational"},
			IssueID:  "ConvertToWebp", Brief: "Large PNG could be smaller as WebP",
			Category: ALCIcons, ALSeverity: ALSInformational, Priority: 3,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
	{
		r := &IconMissingDensityFolderRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "IconMissingDensityFolder", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "IconMissingDensityFolder", Brief: "Missing density folder",
			Category: ALCIcons, ALSeverity: ALSWarning, Priority: 3,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
	{
		r := &IconExpectedSizeRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "IconExpectedSize", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "IconExpectedSize", Brief: "Launcher icon not at expected size",
			Category: ALCIcons, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
	{
		r := &IconNoDpiRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "IconNoDpi", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "IconNoDpi", Brief: "Icon in both nodpi and density-specific folder",
			Category: ALCIcons, ALSeverity: ALSWarning, Priority: 4,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
	{
		r := &IconDuplicatesConfigRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "IconDuplicatesConfig", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "IconDuplicatesConfig", Brief: "Identical icons across configuration folders",
			Category: ALCIcons, ALSeverity: ALSWarning, Priority: 3,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			AndroidDeps: uint32(AndroidDepIcons), Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {},
		})
	}
}
