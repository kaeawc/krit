package rules

import "github.com/kaeawc/krit/internal/oracle"

// BuildOracleFilterRules converts a slice of enabled Rule values into the
// minimal OracleFilterRule representation consumed by
// oracle.CollectOracleFiles. This bridge exists because the oracle
// package intentionally does not import the rules package; the rules
// package is the natural place to own the rules -> filter-spec
// adaptation so that oracle.CollectOracleFiles can stay dependency-free.
//
// Any rule without an explicit OracleFilter() method receives the
// conservative AllFiles: true default via GetOracleFilter, which means
// CollectOracleFiles will short-circuit to "all files" as soon as the
// first unclassified rule is seen. That is the intended behavior until
// more rules are audited.
func BuildOracleFilterRules(enabled []Rule) []oracle.OracleFilterRule {
	out := make([]oracle.OracleFilterRule, 0, len(enabled))
	for _, r := range enabled {
		f := GetOracleFilter(r)
		spec := &oracle.OracleFilterSpec{
			Identifiers: f.Identifiers,
			AllFiles:    f.AllFiles,
		}
		out = append(out, oracle.OracleFilterRule{Name: r.Name(), Filter: spec})
	}
	return out
}
