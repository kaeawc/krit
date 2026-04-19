package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// MetaForRule returns the RuleDescriptor for a registered rule.
//
// It first tries Unwrap(r) and asserts the concrete struct implements
// registry.MetaProvider — the fast path for WrapAsV2-registered rules
// (and AdaptFlatDispatch-registered rules that set AdaptWithOriginalV1).
//
// For adapter-wrapped rules that dropped the concrete pointer, Unwrap
// returns the v2 compat wrapper, which does not implement MetaProvider.
// In that case we fall back to the generated metaByName index (keyed by
// rule ID) so ApplyConfig can still honor ruleset/rule active overrides
// and schema generation can publish the rule's options.
//
// The second return is false when the rule name is not covered by the
// index. This happens for alias registrations whose ID is not the
// primary struct ID (the 4 known aliases live in alias_meta.go).
func MetaForRule(r Rule) (registry.RuleDescriptor, bool) {
	concrete := Unwrap(r)
	if mp, ok := concrete.(registry.MetaProvider); ok {
		m := mp.Meta()
		if m.ID == r.Name() {
			return m, true
		}
	}
	if m, ok := metaByName()[r.Name()]; ok {
		return m, true
	}
	return registry.RuleDescriptor{}, false
}

// MetaForV2Rule returns the RuleDescriptor for a v2 rule.
//
// It first checks OriginalV1 for a registry.MetaProvider (fast path for
// adapter-wrapped rules that preserved the concrete struct pointer via
// AdaptWithOriginalV1). It then falls back to the generated metaByName
// index keyed by rule ID.
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
