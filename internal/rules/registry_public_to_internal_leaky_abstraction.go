package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPublicToInternalLeakyAbstractionRules() {

	// --- from public_to_internal_leaky_abstraction.go ---
	{
		r := &PublicToInternalLeakyAbstractionRule{
			BaseRule:  BaseRule{RuleName: "PublicToInternalLeakyAbstraction", RuleSetName: "architecture", Sev: "info", Desc: "Flags public classes that are thin wrappers delegating to a single private or internal field, which leak internal abstractions through a nominally public API."},
			Threshold: 0.80,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
