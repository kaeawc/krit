package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerLayerDependencyViolationRules() {

	// --- from layer_dependency_violation.go ---
	{
		r := &LayerDependencyViolationRule{
			BaseRule: BaseRule{RuleName: "LayerDependencyViolation", RuleSetName: "architecture", Sev: "warning", Desc: "Flags Gradle module dependencies that cross architectural layer boundaries not permitted by the configured layer matrix."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsModuleIndex, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
