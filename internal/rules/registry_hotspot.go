package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerHotspotRules() {

	// --- from hotspot.go ---
	{
		r := &GodClassOrModuleRule{
			BaseRule:                BaseRule{RuleName: "GodClassOrModule", RuleSetName: "architecture", Sev: "warning", Desc: "Detects source files that import from an unusually broad set of packages, suggesting too many responsibilities."},
			AllowedDistinctPackages: 12,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &FanInFanOutHotspotRule{
			BaseRule:                BaseRule{RuleName: "FanInFanOutHotspot", RuleSetName: "architecture", Sev: "info", Desc: "Detects class-like declarations with unusually high fan-in across the project."},
			AllowedFanIn:            20,
			IgnoreCommentReferences: true,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsCrossFile, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
