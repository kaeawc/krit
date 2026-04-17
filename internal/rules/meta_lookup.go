package rules

import "github.com/kaeawc/krit/internal/rules/registry"

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
