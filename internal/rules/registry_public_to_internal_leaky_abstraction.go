package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerPublicToInternalLeakyAbstractionRules() {

	// --- from public_to_internal_leaky_abstraction.go ---
	{
		r := &PublicToInternalLeakyAbstractionRule{
			BaseRule:  BaseRule{RuleName: "PublicToInternalLeakyAbstraction", RuleSetName: "architecture", Sev: "info", Desc: "Flags public classes that are thin wrappers delegating to a single private or internal field, which leak internal abstractions through a nominally public API."},
			Threshold: 0.80,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, OriginalV1: r,
			Check: r.check,
		})
	}
}
