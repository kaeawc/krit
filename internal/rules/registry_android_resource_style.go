package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidResourceStyleRules() {

	// --- from android_resource_style.go ---
	{
		r := &PxUsageResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "PxUsageResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "PxUsage",
			Brief:      "Using px instead of dp in dimensions",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &SpUsageResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "SpUsageResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "SpUsage",
			Brief:      "Using dp instead of sp for textSize",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &SmallSpResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "SmallSpResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "SmallSp",
			Brief:      "Text size below 12sp",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &InOrMmUsageResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InOrMmUsageResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "InOrMmUsage",
			Brief:      "Using in or mm dimension units",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NegativeMarginResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "NegativeMarginResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "NegativeMargin",
			Brief:      "Negative margin value",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &Suspicious0dpResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "Suspicious0dpResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "Suspicious0dp",
			Brief:      "0dp dimension on wrong axis in LinearLayout",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: 0.75, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &DisableBaselineAlignmentResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "DisableBaselineAlignmentResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "DisableBaselineAlignment",
			Brief:      "Missing baselineAligned=false on weighted LinearLayout",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &InefficientWeightResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InefficientWeightResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "InefficientWeight",
			Brief:      "LinearLayout with weights missing orientation",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NestedWeightsResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "NestedWeightsResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "NestedWeights",
			Brief:      "Nested layout_weight causes exponential measure passes",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ObsoleteLayoutParamsResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ObsoleteLayoutParamsResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ObsoleteLayoutParam",
			Brief:      "layout_weight on non-LinearLayout child",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MergeRootFrameResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "MergeRootFrameResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "MergeRootFrame",
			Brief:      "Root FrameLayout replaceable with merge tag",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &OverdrawResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "OverdrawResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "Overdraw",
			Brief:      "Root and child layout both have background (overdraw)",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &AlwaysShowActionResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "AlwaysShowActionResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "AlwaysShowAction",
			Brief:      "showAsAction=always can crowd the action bar",
			Category:   ALCUsability,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &StateListReachableResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "StateListReachableResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "StateListReachable",
			Brief:      "Unreachable item in selector drawable",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
