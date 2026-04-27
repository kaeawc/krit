package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerModuleDependencyCycleRules() {

	// --- from module_dependency_cycle.go ---
	{
		r := &ModuleDependencyCycleRule{
			BaseRule: BaseRule{RuleName: "ModuleDependencyCycle", RuleSetName: "architecture", Sev: "warning", Desc: "Detects cycles in the Gradle module dependency graph (e.g. :a → :b → :c → :a)."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsModuleIndex, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
