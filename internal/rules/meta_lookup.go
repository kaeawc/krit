package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// MetaForV2Rule returns the RuleDescriptor for a v2 rule.
//
// It first checks Implementation for a registry.MetaProvider (fast path for
// rules that set the concrete struct pointer on their v2.Rule). It then
// falls back to the metaByName index keyed by rule ID.
func MetaForV2Rule(r *v2.Rule) (registry.RuleDescriptor, bool) {
	if r == nil {
		return registry.RuleDescriptor{}, false
	}
	if r.Implementation != nil {
		if mp, ok := r.Implementation.(registry.MetaProvider); ok {
			m := mp.Meta()
			if m.ID == r.ID {
				return m, true
			}
		}
	}
	if m, ok := metaByName()[r.ID]; ok {
		return m, true
	}
	return registry.RuleDescriptor{}, false
}

// HasV2Implementation reports whether a v2 rule has both executable analysis
// logic and a declared dispatcher route.
func HasV2Implementation(r *v2.Rule) bool {
	if r == nil {
		return false
	}
	hasCheck := r.Check != nil
	hasAggregate := r.Needs.Has(v2.NeedsAggregate) &&
		r.Aggregate != nil &&
		r.Aggregate.Collect != nil &&
		r.Aggregate.Finalize != nil &&
		r.Aggregate.Reset != nil
	hasRoute := len(r.NodeTypes) > 0 ||
		r.NodeTypes == nil ||
		r.Needs != 0 ||
		r.AndroidDeps != 0
	return (hasCheck || hasAggregate) && hasRoute
}

// V2RulePrecision returns the dominant precision class for a v2 rule.
func V2RulePrecision(r *v2.Rule) Precision {
	if r == nil {
		return PrecisionHeuristicTextBacked
	}
	if policyRuleNames[r.ID] {
		return PrecisionPolicy
	}
	if heuristicRuleNames[r.ID] {
		return PrecisionHeuristicTextBacked
	}
	if r.Needs.Has(v2.NeedsManifest) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(v2.NeedsGradle) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(v2.NeedsResources) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(v2.NeedsParsedFiles) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(v2.NeedsCrossFile) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(v2.NeedsModuleIndex) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(v2.NeedsResolver) {
		return PrecisionTypeAware
	}
	if r.Implementation != nil {
		if _, ok := r.Implementation.(interface {
			SetResolver(typeinfer.TypeResolver)
		}); ok {
			return PrecisionTypeAware
		}
	}
	if len(r.NodeTypes) > 0 {
		return PrecisionASTBacked
	}
	if r.Needs.Has(v2.NeedsAggregate) {
		return PrecisionASTBacked
	}
	if r.Needs.Has(v2.NeedsLinePass) {
		return PrecisionHeuristicTextBacked
	}
	return PrecisionHeuristicTextBacked
}
