package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerAndroidResourceLayoutRules() {

	// --- from android_resource_layout.go ---
	{
		r := &TooManyViewsResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "TooManyViewsResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "TooManyViews",
			Brief:      "Layout has too many views",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   1,
			Origin:     "AOSP Android Lint",
		}, MaxViews: 80}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &TooDeepLayoutResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "TooDeepLayoutResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "TooDeepLayout",
			Brief:      "Layout nesting too deep",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   1,
			Origin:     "AOSP Android Lint",
		}, MaxDepth: 10}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &UselessParentResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UselessParentResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UselessParent",
			Brief:      "Useless parent layout with single child",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   2,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &UselessLeafResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UselessLeafResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UselessLeaf",
			Brief:      "Empty ViewGroup with no background or id",
			Category:   ALCPerformance,
			ALSeverity: ALSWarning,
			Priority:   2,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NestedScrollingResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "NestedScrollingResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "NestedScrolling",
			Brief:      "ScrollView inside ScrollView",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ScrollViewCountResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ScrollViewCountResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "ScrollViewCount",
			Brief:      "ScrollView with more than one child",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ScrollViewSizeResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "ScrollViewSizeResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "ScrollViewSize",
			Brief:      "ScrollView size validation",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   7,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &RequiredSizeResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RequiredSizeResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "RequiredSize",
			Brief:      "View missing layout_width or layout_height",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
			Priority:   8,
			Origin:     "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &OrientationResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "OrientationResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "Orientation",
			Brief:      "LinearLayout missing explicit orientation",
			Category:   ALCCorrectness,
			ALSeverity: ALSError,
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
		r := &AdapterViewChildrenResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "AdapterViewChildrenResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "AdapterViewChildren",
			Brief:      "AdapterView cannot have children in XML",
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
		r := &IncludeLayoutParamResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "IncludeLayoutParamResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "IncludeLayoutParam",
			Brief:      "<include> with layout_width/height is ignored",
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
	{
		r := &UseCompoundDrawablesResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "UseCompoundDrawablesResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "UseCompoundDrawables",
			Brief:      "Node can be replaced by a TextView with compound drawables",
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
		r := &InconsistentLayoutResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "InconsistentLayout", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "InconsistentLayout",
			Brief:      "Inconsistent layouts in different configurations",
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
}
