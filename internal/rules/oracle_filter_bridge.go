package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// BuildOracleFilterRulesV2 converts the subset of v2 rules that declare
// NeedsOracle into the minimal OracleFilterRule representation consumed
// by oracle.CollectOracleFiles.
//
// Inversion semantics (roadmap/core-infra/oracle-filter-inversion.md):
// rules that do NOT declare NeedsOracle are excluded from the oracle
// selection entirely — the oracle is only invoked on files an
// oracle-needing rule asked for. A rule that declares NeedsOracle with
// no Oracle filter set (or an AllFiles: true filter) is treated as
// wanting every file.
func BuildOracleFilterRulesV2(enabled []*v2.Rule) []oracle.OracleFilterRule {
	out := make([]oracle.OracleFilterRule, 0, len(enabled))
	for _, r := range enabled {
		if r == nil {
			continue
		}
		if !r.Needs.Has(v2.NeedsOracle) {
			continue
		}
		var spec *oracle.OracleFilterSpec
		if r.Oracle != nil {
			spec = &oracle.OracleFilterSpec{
				Identifiers: r.Oracle.Identifiers,
				AllFiles:    r.Oracle.AllFiles,
			}
		} else {
			// The rule declared NeedsOracle but did not narrow by
			// content — it wants every file.
			spec = &oracle.OracleFilterSpec{AllFiles: true}
		}
		out = append(out, oracle.OracleFilterRule{Name: r.ID, Filter: spec})
	}
	return out
}

// BuildOracleCallTargetFilterV2 unions the call-target interest declared by
// active rules. If any enabled oracle rule declares AllCalls, the returned
// summary is disabled and callers must preserve the old "resolve every call"
// behavior. Rules with nil OracleCallTargets are treated as non-consumers of
// LookupCallTarget and do not contribute to the union.
func BuildOracleCallTargetFilterV2(enabled []*v2.Rule) oracle.CallTargetFilterSummary {
	return BuildOracleCallTargetFilterV2ForFiles(enabled, nil)
}

// OracleCallTargetFilterNeedsFiles reports whether any active rule asks
// the call-target filter to derive lexical callee names from source files.
func OracleCallTargetFilterNeedsFiles(enabled []*v2.Rule) bool {
	for _, r := range enabled {
		if r == nil || !r.Needs.Has(v2.NeedsOracle) || r.OracleCallTargets == nil {
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
// the JVM preserves the old broad LookupCallTarget behavior.
func BuildOracleCallTargetFilterV2ForFiles(enabled []*v2.Rule, files []*scanner.File) oracle.CallTargetFilterSummary {
	summary := oracle.CallTargetFilterSummary{Enabled: true}
	for _, r := range enabled {
		if r == nil || !r.Needs.Has(v2.NeedsOracle) {
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
