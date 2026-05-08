package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerModuleDependencyCycleRules() {

	// --- from module_dependency_cycle.go ---
	{
		r := &ModuleDependencyCycleRule{
			BaseRule: BaseRule{RuleName: "ModuleDependencyCycle", RuleSetName: "architecture", Sev: "warning", Desc: "Detects cycles in the Gradle module dependency graph (e.g. :a → :b → :c → :a)."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: true,
		})
	}
}
