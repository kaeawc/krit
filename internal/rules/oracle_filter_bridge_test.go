package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func TestBuildOracleFilterRulesV2_SkipsRulesWithoutNeedsOracle(t *testing.T) {
	rules := []*v2.Rule{
		{ID: "SyntacticRule"},                         // no NeedsOracle → excluded
		{ID: "ResolverOnly", Needs: v2.NeedsResolver}, // no NeedsOracle → excluded
		{ID: "OracleAll", Needs: v2.NeedsOracle},      // included, AllFiles default
		{ID: "OracleFiltered", Needs: v2.NeedsResolver | v2.NeedsOracle,
			Oracle: &v2.OracleFilter{Identifiers: []string{"suspend"}}},
		// NeedsTypeInfo subsumes NeedsOracle → included.
		{ID: "TypeInfoAll", Needs: v2.NeedsTypeInfo},
		{ID: "TypeInfoFiltered", Needs: v2.NeedsTypeInfo,
			Oracle: &v2.OracleFilter{Identifiers: []string{"!!"}}},
	}
	got := BuildOracleFilterRulesV2(rules)
	if len(got) != 4 {
		t.Fatalf("got %d rules, want 4 (only NeedsOracle / NeedsTypeInfo rules pass through)", len(got))
	}

	byName := map[string]oracle.OracleFilterRule{}
	for _, r := range got {
		byName[r.Name] = r
	}
	for _, want := range []string{"OracleAll", "OracleFiltered", "TypeInfoAll", "TypeInfoFiltered"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("%s missing from filter set: %+v", want, byName)
		}
	}
	for _, skip := range []string{"SyntacticRule", "ResolverOnly"} {
		if _, leaked := byName[skip]; leaked {
			t.Errorf("non-oracle rule leaked through: %s", skip)
		}
	}

	for _, name := range []string{"OracleAll", "TypeInfoAll"} {
		r := byName[name]
		if r.Filter == nil || !r.Filter.AllFiles {
			t.Errorf("%s: Filter=%+v, want AllFiles:true default", name, r.Filter)
		}
	}
	wantIDs := map[string]string{"OracleFiltered": "suspend", "TypeInfoFiltered": "!!"}
	for name, want := range wantIDs {
		r := byName[name]
		if r.Filter == nil || r.Filter.AllFiles ||
			len(r.Filter.Identifiers) != 1 || r.Filter.Identifiers[0] != want {
			t.Errorf("%s: Filter=%+v, want Identifiers:[%s]", name, r.Filter, want)
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

