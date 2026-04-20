package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// BuildOracleFilterRulesV2 converts a slice of v2 rules directly into the
// minimal OracleFilterRule representation consumed by oracle.CollectOracleFiles,
// skipping the v2→v1 roundtrip.
func BuildOracleFilterRulesV2(enabled []*v2.Rule) []oracle.OracleFilterRule {
	out := make([]oracle.OracleFilterRule, 0, len(enabled))
	for _, r := range enabled {
		if r == nil {
			continue
		}
		var spec *oracle.OracleFilterSpec
		if r.Oracle != nil {
			spec = &oracle.OracleFilterSpec{
				Identifiers: r.Oracle.Identifiers,
				AllFiles:    r.Oracle.AllFiles,
			}
		} else {
			// Conservative default: no filter declared means all files.
			spec = &oracle.OracleFilterSpec{AllFiles: true}
		}
		out = append(out, oracle.OracleFilterRule{Name: r.ID, Filter: spec})
	}
	return out
}
