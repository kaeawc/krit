package rules

import (
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func TestBuildOracleFilterRulesV2_SkipsRulesWithoutNeedsOracle(t *testing.T) {
	rules := []*v2.Rule{
		{ID: "SyntacticRule"},                               // no NeedsOracle → excluded
		{ID: "ResolverOnly", Needs: v2.NeedsResolver},       // no NeedsOracle → excluded
		{ID: "OracleAll", Needs: v2.NeedsOracle},            // included, AllFiles default
		{ID: "OracleFiltered", Needs: v2.NeedsResolver | v2.NeedsOracle,
			Oracle: &v2.OracleFilter{Identifiers: []string{"suspend"}}},
	}
	got := BuildOracleFilterRulesV2(rules)
	if len(got) != 2 {
		t.Fatalf("got %d rules, want 2 (only NeedsOracle rules should pass through)", len(got))
	}

	byName := map[string]bool{}
	for _, r := range got {
		byName[r.Name] = true
	}
	if !byName["OracleAll"] || !byName["OracleFiltered"] {
		t.Errorf("expected OracleAll and OracleFiltered; got %+v", byName)
	}
	if byName["SyntacticRule"] || byName["ResolverOnly"] {
		t.Errorf("non-oracle rules leaked through: %+v", byName)
	}

	for _, r := range got {
		if r.Name == "OracleAll" {
			if r.Filter == nil || !r.Filter.AllFiles {
				t.Errorf("OracleAll: Filter=%+v, want AllFiles:true default", r.Filter)
			}
		}
		if r.Name == "OracleFiltered" {
			if r.Filter == nil || r.Filter.AllFiles ||
				len(r.Filter.Identifiers) != 1 || r.Filter.Identifiers[0] != "suspend" {
				t.Errorf("OracleFiltered: Filter=%+v, want Identifiers:[suspend]", r.Filter)
			}
		}
	}
}

func TestBuildOracleFilterRulesV2_NoOracleRulesReturnsEmpty(t *testing.T) {
	rules := []*v2.Rule{
		{ID: "A"},
		{ID: "B", Needs: v2.NeedsResolver},
		{ID: "C", Needs: v2.NeedsLinePass},
	}
	got := BuildOracleFilterRulesV2(rules)
	if len(got) != 0 {
		t.Errorf("got %d rules, want 0 — no rule declared NeedsOracle", len(got))
	}
}
