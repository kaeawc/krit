package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// MetaForRule returns the RuleDescriptor for a rule.
//
// Every registered rule has a descriptor: ID, RuleSet, Severity,
// Description, and Confidence are taken from r (the registry is the
// source of truth for those). The remaining fields (DefaultActive,
// FixLevel, Options, CustomApply, LanguageSupport) are read directly
// from r when populated, otherwise from the rule's optional
// MetaProvider implementation, otherwise from the metaByName index.
// The dual path supports the in-progress migration from per-rule
// Meta() methods to descriptor fields on api.Rule itself; once every
// rule has migrated, the MetaProvider fallback can be deleted.
//
// Returns (zero, false) only when r is nil — every non-nil registered
// rule yields a descriptor.
func MetaForRule(r *api.Rule) (api.RuleDescriptor, bool) {
	if r == nil {
		return api.RuleDescriptor{}, false
	}
	extra := metaExtraFor(r)
	return withRuleLanguageSupport(r, mergeRuleDescriptor(r, extra)), true
}

// metaExtraFor returns the rule's MetaProvider descriptor when present,
// or a zero descriptor otherwise. Used by MetaForRule during the
// migration period — once all rules carry their descriptor on api.Rule
// directly, this function and the metaByName fallback go away.
func metaExtraFor(r *api.Rule) api.RuleDescriptor {
	if r.Implementation != nil {
		if mp, ok := r.Implementation.(api.MetaProvider); ok {
			return mp.Meta()
		}
	}
	if m, ok := metaByName()[r.ID]; ok {
		return m
	}
	return api.RuleDescriptor{}
}

// mergeRuleDescriptor builds a RuleDescriptor by taking authoritative
// fields from r and combining them with extra's unique fields. For
// fields that may live on either r or extra (the migration set), r
// wins when populated and extra is the fallback.
func mergeRuleDescriptor(r *api.Rule, extra api.RuleDescriptor) api.RuleDescriptor {
	out := api.RuleDescriptor{
		ID:            r.ID,
		RuleSet:       r.Category,
		Severity:      string(r.Sev),
		Description:   r.Description,
		Confidence:    r.Confidence,
		DefaultActive: r.DefaultActive || extra.DefaultActive,
		FixLevel:      extra.FixLevel,
	}
	if r.Options != nil {
		out.Options = r.Options
	} else {
		out.Options = extra.Options
	}
	if r.CustomApply != nil {
		out.CustomApply = r.CustomApply
	} else {
		out.CustomApply = extra.CustomApply
	}
	if len(r.LanguageSupport) > 0 {
		out.LanguageSupport = r.LanguageSupport
	} else {
		out.LanguageSupport = extra.LanguageSupport
	}
	if out.FixLevel == "" && r.Fix != api.FixNone {
		out.FixLevel = r.Fix.String()
	}
	if len(r.Aliases) > 0 {
		out.Aliases = r.Aliases
	} else {
		out.Aliases = extra.Aliases
	}
	if r.EnabledByDefaultSince != "" {
		out.EnabledByDefaultSince = r.EnabledByDefaultSince
	} else {
		out.EnabledByDefaultSince = extra.EnabledByDefaultSince
	}
	if r.Deprecated != nil {
		out.Deprecated = r.Deprecated
	} else {
		out.Deprecated = extra.Deprecated
	}
	if len(r.Tags) > 0 {
		out.Tags = r.Tags
	} else {
		out.Tags = extra.Tags
	}
	out.Precision = resolvePrecision(r, extra)
	return out
}

// resolvePrecision picks the tier returned in a descriptor. A
// MetaProvider-supplied extra.Precision only wins when the rule itself
// declared no explicit override; otherwise V2RulePrecision is the
// single source of truth (it already honors Rule.Precision and
// PrecisionProvider). V2RulePrecision never returns PrecisionUnset, so
// MetaForRule always emits a populated tier.
func resolvePrecision(r *api.Rule, extra api.RuleDescriptor) api.Precision {
	if r.Precision == api.PrecisionUnset && extra.Precision != api.PrecisionUnset {
		return extra.Precision
	}
	return V2RulePrecision(r)
}

func withRuleLanguageSupport(r *api.Rule, m api.RuleDescriptor) api.RuleDescriptor {
	support, ok := JavaSupportForRule(r)
	if !ok {
		return m
	}
	if m.LanguageSupport == nil {
		m.LanguageSupport = map[string]api.LanguageSupport{}
	} else {
		copied := make(map[string]api.LanguageSupport, len(m.LanguageSupport)+1)
		for lang, existing := range m.LanguageSupport {
			copied[lang] = existing
		}
		m.LanguageSupport = copied
	}
	if _, exists := m.LanguageSupport[JavaLanguageSupportKey]; !exists {
		m.LanguageSupport[JavaLanguageSupportKey] = support
	}
	return m
}

// HasV2Implementation reports whether a rule has both executable analysis
// logic and a declared dispatcher route.
func HasV2Implementation(r *api.Rule) bool {
	if r == nil {
		return false
	}
	hasCheck := r.Check != nil
	hasAggregate := r.Needs.Has(api.NeedsAggregate) &&
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

// V2RulePrecision returns the dominant precision class for a rule.
//
// Resolution order:
//  1. Rule.Precision when set (non-zero) — the rule has overridden the
//     derived tier.
//  2. Rule.Implementation when it implements api.PrecisionProvider —
//     lets tests stub a tier without touching the Rule literal.
//  3. Derived from rule shape (Needs / NodeTypes / known override maps).
func V2RulePrecision(r *api.Rule) Precision {
	if r == nil {
		return PrecisionHeuristicTextBacked
	}
	if r.Precision != PrecisionUnset {
		return r.Precision
	}
	if r.Implementation != nil {
		if pp, ok := r.Implementation.(api.PrecisionProvider); ok {
			if p := pp.Precision(); p != PrecisionUnset {
				return p
			}
		}
	}
	if policyRuleNames[r.ID] {
		return PrecisionPolicy
	}
	if heuristicRuleNames[r.ID] {
		return PrecisionHeuristicTextBacked
	}
	if r.Needs.Has(api.NeedsManifest) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(api.NeedsGradle) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(api.NeedsResources) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(api.NeedsParsedFiles) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(api.NeedsCrossFile) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(api.NeedsModuleIndex) {
		return PrecisionProjectStructure
	}
	if r.Needs.Has(api.NeedsResolver) {
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
	if r.Needs.Has(api.NeedsAggregate) {
		return PrecisionASTBacked
	}
	if r.Needs.Has(api.NeedsLinePass) {
		return PrecisionHeuristicTextBacked
	}
	return PrecisionHeuristicTextBacked
}
