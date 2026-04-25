package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// MetaForV2Rule returns the RuleDescriptor for a v2 rule.
//
// It first checks OriginalV1 for a registry.MetaProvider (fast path for
// rules that set the concrete struct pointer on their v2.Rule). It then
// falls back to the metaByName index keyed by rule ID.
func MetaForV2Rule(r *v2.Rule) (registry.RuleDescriptor, bool) {
	if r == nil {
		return registry.RuleDescriptor{}, false
	}
	if r.OriginalV1 != nil {
		if mp, ok := r.OriginalV1.(registry.MetaProvider); ok {
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

// IsImplementedV2 reports whether a v2 rule has a real implementation path.
// Rules without node types, line pass, or android deps are pure stubs.
func IsImplementedV2(r *v2.Rule) bool {
	if r == nil {
		return false
	}
	return len(r.NodeTypes) > 0 || r.Needs != 0 || r.AndroidDeps != 0
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
	if r.OriginalV1 != nil {
		if _, ok := r.OriginalV1.(interface {
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
