package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// RuleNeedsKotlinOracle reports whether a rule is an actual KAA consumer.
// NeedsTypeInfo is intentionally not enough: it is resolver-only source type
// information. Oracle participation must come from any narrow NeedsOracle*
// bit, the legacy NeedsOracle umbrella, or the explicit rule metadata
// (Oracle / OracleCallTargets / OracleDeclarationNeeds) that the pipeline
// passes to krit-types.
func RuleNeedsKotlinOracle(r *api.Rule) bool {
	if r == nil {
		return false
	}
	if r.Needs.HasAny(api.NeedsOracle) {
		return true
	}
	if r.Oracle != nil || r.OracleCallTargets != nil || r.OracleDeclarationNeeds != nil {
		return true
	}
	return ruleNeedsOracleDiagnostics(r)
}

// OracleFactUnion returns the OR of fact-category bits across the
// active rule set, lifting legacy metadata (Oracle / OracleCallTargets /
// OracleDeclarationNeeds / hardcoded diagnostic rule IDs) into the
// matching narrow bits. This is the single place rule descriptors are
// translated into the JVM workload mask consumed by InvocationOptions.
//
// Rules with NeedsOracle (umbrella) contribute every narrow bit. Rules
// with only legacy metadata contribute the bits implied by that
// metadata; rules that declared narrow bits contribute exactly those.
// Active rules without any oracle interest contribute zero.
func OracleFactUnion(enabled []*api.Rule) api.Capabilities {
	var union api.Capabilities
	for _, r := range enabled {
		union |= ruleOracleFactBits(r)
	}
	return union
}

// ruleOracleFactBits returns the fact-category bits one rule
// contributes, including the back-compat shim that lifts legacy
// metadata into bits.
func ruleOracleFactBits(r *api.Rule) api.Capabilities {
	if r == nil {
		return 0
	}
	bits := r.Needs & api.NeedsOracle

	// Legacy: any non-bit oracle metadata implies the umbrella unless
	// the rule has narrowed via OracleDeclarationNeeds. We expand
	// metadata into the bits the metadata semantically asserts; any
	// remaining ambiguity (Oracle filter only, no narrowing) is treated
	// conservatively as the umbrella so behavior is preserved during
	// the migration.
	if r.OracleCallTargets != nil {
		bits |= api.NeedsOracleCallTargets
	}
	if r.OracleDeclarationNeeds != nil {
		n := r.OracleDeclarationNeeds
		if n.Supertypes {
			bits |= api.NeedsOracleSupertypes
		}
		if n.ClassAnnotations {
			bits |= api.NeedsOracleClassAnnotations
		}
		if n.Members {
			bits |= api.NeedsOracleMembers
		}
		if n.MemberSignatures {
			bits |= api.NeedsOracleMembers | api.NeedsOracleMemberSignatures
		}
		if n.MemberAnnotations {
			bits |= api.NeedsOracleMembers | api.NeedsOracleMemberAnnotations
		}
		if n.SourceDependencyClosure {
			bits |= api.NeedsOracleLibraryClasses
		}
	}
	// Oracle: the legacy file-selection filter does not by itself imply
	// any narrow fact category (it gates which files are sent to KAA,
	// not what is extracted). When it appears alongside no narrowing
	// metadata at all, conservatively expand to the umbrella so we do
	// not silently downgrade an unmigrated rule. A non-nil
	// OracleDeclarationNeeds (even empty) counts as explicit narrowing.
	if r.Oracle != nil && bits == 0 &&
		r.OracleCallTargets == nil &&
		r.OracleDeclarationNeeds == nil {
		bits |= api.NeedsOracle
	}
	if ruleNeedsOracleDiagnostics(r) {
		bits |= api.NeedsOracleDiagnostics
	}
	return bits
}

// NeedsOracleDeclarationWalk reports whether any active rule consumes a
// fact that requires the JVM-side declaration walk (members, signatures,
// supertypes, class/member annotations). When false, krit-types can
// skip declaration extraction entirely.
func NeedsOracleDeclarationWalk(enabled []*api.Rule) bool {
	declarationBits := api.NeedsOracleSupertypes |
		api.NeedsOracleMembers |
		api.NeedsOracleMemberSignatures |
		api.NeedsOracleClassAnnotations |
		api.NeedsOracleMemberAnnotations
	return OracleFactUnion(enabled).HasAny(declarationBits)
}

// NeedsOracleLibraryClasses reports whether any active rule needs the
// JAR / library closure (Dependencies map). When false, krit-types can
// skip the library walk.
func NeedsOracleLibraryClasses(enabled []*api.Rule) bool {
	return OracleFactUnion(enabled).HasAny(api.NeedsOracleLibraryClasses)
}

// KotlinOracleRulesV2 returns the active subset that should contribute to KAA
// file selection, call-target filtering, declaration export, and diagnostics.
func KotlinOracleRulesV2(enabled []*api.Rule) []*api.Rule {
	out := make([]*api.Rule, 0, len(enabled))
	for _, r := range enabled {
		if RuleNeedsKotlinOracle(r) {
			out = append(out, r)
		}
	}
	return out
}

// BuildOracleFilterRulesV2 converts the subset of v2 rules that need the
// Kotlin oracle into the minimal OracleFilterRule representation consumed
// by oracle.CollectOracleFiles.
//
// Inversion semantics (roadmap/core-infra/oracle-filter-inversion.md):
// rules that do NOT need the oracle are excluded from oracle selection
// entirely — the oracle is only invoked on files an oracle-needing rule
// asked for. A rule that needs the oracle with no Oracle filter set (or
// an AllFiles: true filter) is treated as
// wanting every file.
func BuildOracleFilterRulesV2(enabled []*api.Rule) []oracle.FilterRule {
	out := make([]oracle.FilterRule, 0, len(enabled))
	for _, r := range enabled {
		if !RuleNeedsKotlinOracle(r) {
			continue
		}
		var spec *oracle.FilterSpec
		if r.Oracle != nil {
			spec = &oracle.FilterSpec{
				Identifiers: r.Oracle.Identifiers,
				AllFiles:    r.Oracle.AllFiles,
			}
		} else {
			// The rule needs the oracle but did not narrow by
			// content — it wants every file.
			spec = &oracle.FilterSpec{AllFiles: true}
		}
		out = append(out, oracle.FilterRule{Name: r.ID, Filter: spec})
	}
	return out
}

// BuildOracleDeclarationProfileV2 derives the declaration-export profile
// for a set of active rules. The result is the union of every oracle-needing
// rule's OracleDeclarationNeeds declaration combined with the implications
// of narrow NeedsOracle* bits.
//
// Conservative semantics: if any active oracle rule has a nil
// OracleDeclarationNeeds AND has not narrowed via bits (i.e. it still
// declares the legacy NeedsOracle umbrella or only an Oracle file
// filter), the union is promoted to FullDeclarationProfile — we cannot
// skip fields that an un-annotated rule might silently consume.
//
// A rule that declares narrow bits (e.g. NeedsOracleCallTargets) and
// has nil OracleDeclarationNeeds contributes only the declaration
// fields its bits imply, NOT the full profile. That is what makes the
// bit split tighter than the legacy "any nil → full" semantic.
func BuildOracleDeclarationProfileV2(enabled []*api.Rule) oracle.DeclarationProfileSummary {
	// First pass: check whether any oracle rule still relies on the
	// umbrella (legacy NeedsOracle, or an Oracle file filter without
	// any narrowing). Those rules force the full profile because we
	// cannot tell what they read.
	for _, r := range enabled {
		if !RuleNeedsKotlinOracle(r) {
			continue
		}
		if ruleForcesFullDeclarationProfile(r) {
			return oracle.FinalizeDeclarationProfile(oracle.FullDeclarationProfile())
		}
	}

	// Second pass: union the declared profiles + bit implications.
	var union oracle.DeclarationProfile
	for _, r := range enabled {
		if !RuleNeedsKotlinOracle(r) {
			continue
		}
		union = oracle.MergeDeclarationProfiles(union, ruleDeclarationProfile(r))
	}
	return oracle.FinalizeDeclarationProfile(union)
}

// ruleForcesFullDeclarationProfile reports whether a rule has opted out
// of declaration narrowing — either by declaring the legacy umbrella
// NeedsOracle, or by attaching an Oracle file filter without any
// accompanying bit / OracleDeclarationNeeds narrowing.
func ruleForcesFullDeclarationProfile(r *api.Rule) bool {
	if r == nil {
		return false
	}
	if r.Needs.Has(api.NeedsOracle) {
		return true
	}
	if r.OracleDeclarationNeeds != nil {
		return false
	}
	if r.Needs.HasAny(api.NeedsOracle) {
		// Rule declared narrow bits (subset of NeedsOracle) — bits drive
		// the profile, no full fallback needed.
		return false
	}
	if r.OracleCallTargets != nil {
		// Call-target-only rules never need declaration data unless
		// they say so via OracleDeclarationNeeds.
		return false
	}
	if ruleNeedsOracleDiagnostics(r) {
		// Diagnostics-only rules never need declaration data.
		return false
	}
	if r.Oracle != nil {
		// Legacy: opaque oracle interest, no narrowing. Conservative
		// fallback.
		return true
	}
	return false
}

// ruleDeclarationProfile derives the declaration profile a rule
// contributes, combining explicit OracleDeclarationNeeds with the
// implications of its narrow NeedsOracle* bits.
func ruleDeclarationProfile(r *api.Rule) oracle.DeclarationProfile {
	if r == nil {
		return oracle.DeclarationProfile{}
	}
	var p oracle.DeclarationProfile
	if n := r.OracleDeclarationNeeds; n != nil {
		p = oracle.DeclarationProfile{
			ClassShell:              n.ClassShell,
			Supertypes:              n.Supertypes,
			ClassAnnotations:        n.ClassAnnotations,
			Members:                 n.Members,
			MemberSignatures:        n.MemberSignatures,
			MemberAnnotations:       n.MemberAnnotations,
			SourceDependencyClosure: n.SourceDependencyClosure,
		}
	}
	bits := r.Needs
	if bits.HasAny(api.NeedsOracleSupertypes) {
		p.ClassShell = true
		p.Supertypes = true
	}
	if bits.HasAny(api.NeedsOracleClassAnnotations) {
		p.ClassShell = true
		p.ClassAnnotations = true
	}
	if bits.HasAny(api.NeedsOracleMembers) {
		p.ClassShell = true
		p.Members = true
	}
	if bits.HasAny(api.NeedsOracleMemberSignatures) {
		p.ClassShell = true
		p.Members = true
		p.MemberSignatures = true
	}
	if bits.HasAny(api.NeedsOracleMemberAnnotations) {
		p.ClassShell = true
		p.Members = true
		p.MemberAnnotations = true
	}
	if bits.HasAny(api.NeedsOracleLibraryClasses) {
		p.SourceDependencyClosure = true
	}
	return p
}

// NeedsOracleDiagnostics reports whether active rules should request expensive
// compiler diagnostics from krit-types. Driven by the OracleFactUnion so a
// rule contributes by declaring NeedsOracleDiagnostics (or the umbrella
// NeedsOracle) — the legacy hardcoded rule-ID list is preserved here as a
// transition shim for un-migrated rules.
func NeedsOracleDiagnostics(enabled []*api.Rule) bool {
	return OracleFactUnion(enabled).HasAny(api.NeedsOracleDiagnostics)
}

// ruleNeedsOracleDiagnostics is the legacy rule-ID shim that lifts known
// diagnostic-consuming rules into the NeedsOracleDiagnostics fact bit
// during the migration window. Once every rule in this list has been
// re-tagged with the bit, this function (and the call site in
// ruleOracleFactBits) can be removed.
func ruleNeedsOracleDiagnostics(r *api.Rule) bool {
	if r == nil {
		return false
	}
	if r.Needs.HasAny(api.NeedsOracleDiagnostics) {
		return true
	}
	switch r.ID {
	case "UnsafeCast", "UnreachableCode":
		return true
	}
	return false
}

// BuildOracleCallTargetFilterV2 unions the call-target interest declared by
// active rules. If any enabled oracle rule declares AllCalls, the returned
// summary is disabled and callers must resolve every call. Rules with nil
// OracleCallTargets are treated as non-consumers of
// LookupCallTarget and do not contribute to the union.
func BuildOracleCallTargetFilterV2(enabled []*api.Rule) oracle.CallTargetFilterSummary {
	return BuildOracleCallTargetFilterV2ForFiles(enabled, nil)
}

// OracleCallTargetFilterNeedsFiles reports whether any active rule asks
// the call-target filter to derive lexical callee names from source files.
func OracleCallTargetFilterNeedsFiles(enabled []*api.Rule) bool {
	for _, r := range enabled {
		if !RuleNeedsKotlinOracle(r) || r.OracleCallTargets == nil {
			continue
		}
		if len(r.OracleCallTargets.AnnotatedIdentifiers) > 0 {
			return true
		}
	}
	return false
}

// BuildOracleCallTargetFilterV2ForFiles is BuildOracleCallTargetFilterV2
// plus source-derived callee names for rules that declare
// OracleCallTargetFilter.AnnotatedIdentifiers. If those rules are evaluated
// without source files, the function conservatively disables filtering so
// the JVM preserves broad LookupCallTarget behavior.
func BuildOracleCallTargetFilterV2ForFiles(enabled []*api.Rule, files []*scanner.File) oracle.CallTargetFilterSummary {
	summary := oracle.CallTargetFilterSummary{Enabled: true}
	for _, r := range enabled {
		if !RuleNeedsKotlinOracle(r) {
			continue
		}
		spec := r.OracleCallTargets
		if spec == nil {
			continue
		}
		profile := oracle.CallTargetRuleProfile{
			RuleID:               r.ID,
			AllCalls:             spec.AllCalls,
			DiscardedOnly:        spec.DiscardedOnly,
			CalleeNames:          append([]string(nil), spec.CalleeNames...),
			TargetFQNs:           append([]string(nil), spec.TargetFQNs...),
			LexicalHintsByCallee: cloneLexicalHintsByCallee(spec.LexicalHintsByCallee),
			LexicalSkipByCallee:  cloneLexicalHintsByCallee(spec.LexicalSkipByCallee),
			AnnotatedIdentifiers: append([]string(nil), spec.AnnotatedIdentifiers...),
		}
		if spec.AllCalls {
			summary.Enabled = false
			summary.DisabledBy = append(summary.DisabledBy, r.ID)
			profile.DisabledReason = "allCalls"
			summary.RuleProfiles = append(summary.RuleProfiles, profile)
			continue
		}
		summary.CalleeNames = append(summary.CalleeNames, spec.CalleeNames...)
		summary.TargetFQNs = append(summary.TargetFQNs, spec.TargetFQNs...)
		summary.LexicalHintsByCallee = mergeLexicalHintsByCallee(summary.LexicalHintsByCallee, spec.LexicalHintsByCallee)
		summary.LexicalSkipByCallee = mergeLexicalHintsByCallee(summary.LexicalSkipByCallee, spec.LexicalSkipByCallee)
		if len(spec.AnnotatedIdentifiers) > 0 {
			if len(files) == 0 {
				summary.Enabled = false
				summary.DisabledBy = append(summary.DisabledBy, r.ID)
				profile.DisabledReason = "missingFiles"
				summary.RuleProfiles = append(summary.RuleProfiles, profile)
				continue
			}
			names, uncertain := deriveAnnotatedDeclarationCalleeNames(files, spec.AnnotatedIdentifiers)
			if uncertain {
				summary.Enabled = false
				summary.DisabledBy = append(summary.DisabledBy, r.ID)
				profile.DisabledReason = "uncertainAnnotatedCallees"
				summary.RuleProfiles = append(summary.RuleProfiles, profile)
				continue
			}
			summary.CalleeNames = append(summary.CalleeNames, names...)
			profile.DerivedCalleeNames = append(profile.DerivedCalleeNames, names...)
		}
		summary.RuleProfiles = append(summary.RuleProfiles, profile)
	}
	return oracle.FinalizeCallTargetFilter(summary)
}

func mergeLexicalHintsByCallee(dst, src map[string][]string) map[string][]string {
	for callee, hints := range src {
		if callee == "" || len(hints) == 0 {
			continue
		}
		if dst == nil {
			dst = make(map[string][]string)
		}
		dst[callee] = append(dst[callee], hints...)
	}
	return dst
}

func cloneLexicalHintsByCallee(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for callee, hints := range in {
		out[callee] = append([]string(nil), hints...)
	}
	return out
}
