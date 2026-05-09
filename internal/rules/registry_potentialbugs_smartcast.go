package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPotentialbugsSmartCastRules() {

	// --- from potentialbugs_smartcast.go ---
	{
		r := &SmartCastInvalidatedRule{BaseRule: BaseRule{RuleName: "SmartCastInvalidated", RuleSetName: "potential-bugs", Sev: "error", Desc: "Detects smart-cast variables that are reassigned and then used without re-checking, which the compiler reports as SMARTCAST_IMPOSSIBLE."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: 0.85, Implementation: r,
			Tags:  []string{"precompile"},
			Check: r.check,
		})
	}
}
