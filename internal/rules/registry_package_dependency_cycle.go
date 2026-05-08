package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPackageDependencyCycleRules() {

	// --- from package_dependency_cycle.go ---
	{
		r := &PackageDependencyCycleRule{
			BaseRule: BaseRule{RuleName: "PackageDependencyCycle", RuleSetName: "architecture", Sev: "info", Desc: "Detects cycles in the package-level import graph within a single Gradle module."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
