package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// CostFor returns the rule's effective Cost. When Rule.Cost is set to a
// non-zero value the rule's declaration wins; otherwise the cost is
// derived from the rule's capability bits. Returns CostUnset only when
// r is nil.
func CostFor(r *api.Rule) api.Cost {
	if r == nil {
		return api.CostUnset
	}
	if r.Cost != api.CostUnset {
		return r.Cost
	}
	return DeriveCost(r)
}

// DeriveCost computes a Cost for a rule purely from its declared
// capabilities. It ignores any explicit Rule.Cost so callers that want
// to compare the declared tier against the derived tier (drift checks,
// tests) can do so. The priority order matches the documented presets:
//
//   - JavaFacts / FIR helpers → CostFIR
//   - Any NeedsOracle* bit, or a TypeInfo hint requiring resolved facts
//     → CostOracle
//   - NeedsCrossFile / NeedsModuleIndex / NeedsParsedFiles / NeedsAggregate
//     → CostCrossFile
//   - NeedsManifest / NeedsResources / NeedsGradle → CostAST (per-file
//     project-data rules; their setup cost is amortized once per project,
//     not per rule)
//   - NeedsLinePass → CostLine
//   - everything else (per-file AST dispatch) → CostAST
//
// A rule with no capabilities and no NodeTypes is treated as CostAST —
// the dispatcher will still run it for every node, which is the
// AST-tier workload.
func DeriveCost(r *api.Rule) api.Cost {
	if r == nil {
		return api.CostUnset
	}
	if r.JavaFacts != nil {
		return api.CostFIR
	}
	if r.Needs.HasAny(api.NeedsOracle) {
		return api.CostOracle
	}
	if r.TypeInfo.Required {
		return api.CostOracle
	}
	if r.Needs.Has(api.NeedsCrossFile) ||
		r.Needs.Has(api.NeedsModuleIndex) ||
		r.Needs.Has(api.NeedsParsedFiles) ||
		r.Needs.Has(api.NeedsAggregate) {
		return api.CostCrossFile
	}
	if r.Needs.Has(api.NeedsLinePass) {
		return api.CostLine
	}
	return api.CostAST
}

// FilterByMaxCost returns the subset of rules whose effective Cost is
// at most maxCost. When maxCost is CostUnset the input slice is
// returned unchanged so callers can opt out by passing zero.
func FilterByMaxCost(in []*api.Rule, maxCost api.Cost) []*api.Rule {
	if maxCost == api.CostUnset {
		return in
	}
	out := make([]*api.Rule, 0, len(in))
	for _, r := range in {
		if CostFor(r) <= maxCost {
			out = append(out, r)
		}
	}
	return out
}
