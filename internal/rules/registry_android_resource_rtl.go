package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerAndroidResourceRtlRules() {

	// --- from android_resource_rtl.go ---
	{
		r := &RtlCompatResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RtlCompatResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "RtlCompat",
			Brief:      "Right-to-left text compatibility issues",
			Category:   ALCI18N,
			ALSeverity: ALSError,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Aliases: []string{"RtlCompatManifest"},
			Needs:   api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			NeedsLibraryFacts: true,
			Check:             r.check,
		})
	}
	{
		r := &RtlHardcodedResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RtlHardcodedResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RtlHardcoded",
			Brief:      "Using left/right instead of start/end for RTL (applies unless the including app sets supportsRtl=false or targets below API 17)",
			Category:   ALCI18N,
			ALSeverity: ALSWarning,
			Priority:   5,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &RtlSymmetryResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RtlSymmetryResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RtlSymmetry",
			Brief:      "Asymmetric padding or margin (Left without Right or vice versa), unless the including app sets supportsRtl=false or targets below API 17",
			Category:   ALCI18N,
			ALSeverity: ALSWarning,
			Priority:   4,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &RtlSuperscriptResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RtlSuperscriptResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RtlSuperscript",
			Brief:      "Superscript/subscript may break in RTL (unless the including app sets supportsRtl=false or targets below API 17)",
			Category:   ALCI18N,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &RelativeOverlapResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "RelativeOverlapResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:    "RelativeOverlap",
			Brief:      "Views in RelativeLayout may overlap",
			Category:   ALCCorrectness,
			ALSeverity: ALSWarning,
			Priority:   3,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &NotSiblingResourceRule{AndroidRule: AndroidRule{
			BaseRule:   BaseRule{RuleName: "NotSiblingResource", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:    "NotSibling",
			Brief:      "RelativeLayout Invalid Constraints",
			Category:   ALCCorrectness,
			ALSeverity: ALSFatal,
			Priority:   6,
			Origin:     "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
