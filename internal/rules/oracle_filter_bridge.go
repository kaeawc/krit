package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
	summary := oracle.CallTargetFilterSummary{Enabled: true}
	for _, r := range enabled {
		if r == nil || !r.Needs.Has(v2.NeedsOracle) {
			continue
		}
		spec := r.OracleCallTargets
		if spec == nil {
			continue
		}
		if spec.AllCalls {
			summary.Enabled = false
			summary.DisabledBy = append(summary.DisabledBy, r.ID)
			continue
		}
		summary.CalleeNames = append(summary.CalleeNames, spec.CalleeNames...)
		summary.TargetFQNs = append(summary.TargetFQNs, spec.TargetFQNs...)
	}
	return oracle.FinalizeCallTargetFilter(summary)
}
