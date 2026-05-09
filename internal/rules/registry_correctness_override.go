package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerCorrectnessOverrideRules() {

	// --- from correctness_override.go ---
	{
		r := &OverrideSignatureMismatchRule{BaseRule: BaseRule{RuleName: "OverrideSignatureMismatch", RuleSetName: "potential-bugs", Sev: "error", Desc: "Detects 'override' functions whose parameter count does not match any supertype member with the same name."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.85, Implementation: r,
			Needs: api.NeedsResolver,
			Tags:  []string{"precompile"},
			Check: r.check,
		})
	}
}
