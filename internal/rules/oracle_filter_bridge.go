package rules

import (
	"slices"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// RuleNeedsKotlinOracle reports whether a rule is an actual oracle
// consumer. The narrow NeedsOracle* bits (or the umbrella alias) are
// the single source of truth — rules must declare the fact categories
// they consume.
//
// NeedsTypeInfo is intentionally not enough: it is resolver-only
// source type information.
func RuleNeedsKotlinOracle(r *api.Rule) bool {
	if r == nil {
		return false
	}
	return r.Needs.HasAny(api.NeedsOracle)
}

// OracleFactUnion returns the OR of NeedsOracle* bits across the
// active rule set: the oracle fact categories (call targets, suspend
// markers, supertypes, members, diagnostics, library closure) the
// active rules consume. The pipeline records it as perf telemetry;
// krit-fir always returns every category for the files it analyzes.
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

// KotlinOracleRulesV2 returns the active subset that should contribute to
// oracle file selection.
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
