package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerHotspotRules() {

	// --- from hotspot.go ---
	{
		r := &GodClassOrModuleRule{
			BaseRule:                BaseRule{RuleName: "GodClassOrModule", RuleSetName: "architecture", Sev: "warning", Desc: "Detects source files that import from an unusually broad set of packages, suggesting too many responsibilities."},
			AllowedDistinctPackages: 12,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"source_file"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &FanInFanOutHotspotRule{
			BaseRule:                BaseRule{RuleName: "FanInFanOutHotspot", RuleSetName: "architecture", Sev: "info", Desc: "Detects class-like declarations with unusually high fan-in across the project."},
			AllowedFanIn:            20,
			IgnoreCommentReferences: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
