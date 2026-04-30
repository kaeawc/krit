package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerPackageDependencyCycleRules() {

	// --- from package_dependency_cycle.go ---
	{
		r := &PackageDependencyCycleRule{
			BaseRule: BaseRule{RuleName: "PackageDependencyCycle", RuleSetName: "architecture", Sev: "info", Desc: "Detects cycles in the package-level import graph within a single Gradle module."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
