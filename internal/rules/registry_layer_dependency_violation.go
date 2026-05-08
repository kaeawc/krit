package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerLayerDependencyViolationRules() {

	// --- from layer_dependency_violation.go ---
	{
		r := &LayerDependencyViolationRule{
			BaseRule: BaseRule{RuleName: "LayerDependencyViolation", RuleSetName: "architecture", Sev: "warning", Desc: "Flags Gradle module dependencies that cross architectural layer boundaries not permitted by the configured layer matrix."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
