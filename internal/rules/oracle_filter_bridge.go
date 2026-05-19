package rules

import (
	"slices"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// RuleNeedsKotlinOracle reports whether a rule is an actual KAA
// consumer. The narrow NeedsOracle* bits (or the umbrella alias) are
// the single source of truth — rules must declare the fact categories
// they consume so the bridge can compute a tight workload union.
//
// NeedsTypeInfo is intentionally not enough: it is resolver-only
// source type information. A rule with OracleCallTargets or
// OracleDeclarationNeeds but no NeedsOracle* bit is a registration
// error caught by TestOracleBitsMatchMetadata.
func RuleNeedsKotlinOracle(r *api.Rule) bool {
	if r == nil {
		return false
	}
	return r.Needs.HasAny(api.NeedsOracle)
}

// OracleFactUnion returns the OR of NeedsOracle* bits across the
// active rule set. This is the single place rule descriptors are
// translated into the JVM workload mask consumed by InvocationOptions:
// rules opt into specific KAA fact categories (call targets, suspend
// markers, supertypes, members, diagnostics, library closure) and the
// pipeline's --no-diagnostics / --declaration-profile flags follow.
func OracleFactUnion(enabled []*api.Rule) api.Capabilities {
	var union api.Capabilities
	for _, r := range enabled {
		if r == nil {
			continue
		}
		union |= r.Needs & api.NeedsOracle
	}
	return union
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
// an AllFiles: true filter) is treated as wanting every file.
//
// thorough projects api.OracleFilter.ThoroughOnlyIdentifiers /
// ThoroughOnlyAllFiles into the produced FilterSpec when true; at false
// the thorough-only fields are dropped so balanced/fast pay no extra
// JVM cost. The bridge is the single projection point so oracle.FilterSpec
// stays depth-agnostic.
func BuildOracleFilterRulesV2(enabled []*api.Rule, thorough bool) []oracle.FilterRule {
	out := make([]oracle.FilterRule, 0, len(enabled))
	for _, r := range enabled {
		if !RuleNeedsKotlinOracle(r) {
			continue
		}
		var spec *oracle.FilterSpec
		if r.Oracle != nil {
			ids := r.Oracle.Identifiers
			all := r.Oracle.AllFiles
			if thorough {
				if r.Oracle.ThoroughOnlyAllFiles {
					all = true
				}
				if len(r.Oracle.ThoroughOnlyIdentifiers) > 0 {
					ids = slices.Concat(ids, r.Oracle.ThoroughOnlyIdentifiers)
				}
			}
			spec = &oracle.FilterSpec{Identifiers: ids, AllFiles: all}
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
// for a set of active rules. The result is the union of every
// oracle-needing rule's OracleDeclarationNeeds declaration combined
// with the implications of its narrow NeedsOracle* bits.
//
// A rule that declares the umbrella NeedsOracle (every narrow bit)
// forces the full profile — we cannot tell which declaration fields it
// reads. Rules that declare narrow bits contribute only the
// declaration fields those bits imply.
func BuildOracleDeclarationProfileV2(enabled []*api.Rule) oracle.DeclarationProfileSummary {
	for _, r := range enabled {
		if !RuleNeedsKotlinOracle(r) {
			continue
		}
		if r.Needs.Has(api.NeedsOracle) {
			return oracle.FinalizeDeclarationProfile(oracle.FullDeclarationProfile())
		}
	}
	var union oracle.DeclarationProfile
	for _, r := range enabled {
		if !RuleNeedsKotlinOracle(r) {
			continue
		}
		union = oracle.MergeDeclarationProfiles(union, ruleDeclarationProfile(r))
	}
	return oracle.FinalizeDeclarationProfile(union)
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

// NeedsOracleDiagnostics reports whether active rules should request
// expensive compiler diagnostics from krit-types. Driven by the
// OracleFactUnion: a rule must declare NeedsOracleDiagnostics (or the
// umbrella NeedsOracle) to opt in.
func NeedsOracleDiagnostics(enabled []*api.Rule) bool {
	return OracleFactUnion(enabled).HasAny(api.NeedsOracleDiagnostics)
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
