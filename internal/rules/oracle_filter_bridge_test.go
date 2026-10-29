package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestBuildOracleFilterRulesV2_SkipsRulesWithoutOracleNeed(t *testing.T) {
	rules := []*api.Rule{
		{ID: "SyntacticRule"},                          // no oracle need → excluded
		{ID: "ResolverOnly", Needs: api.NeedsResolver}, // no oracle need → excluded
		{ID: "OracleAll", Needs: api.NeedsOracle},      // included, AllFiles default
		{ID: "OracleFiltered", Needs: api.NeedsResolver | api.NeedsOracle,
			Oracle: &api.OracleFilter{Identifiers: []string{"suspend"}}},
		{ID: "TypeInfoOnly", Needs: api.NeedsTypeInfo},
		{ID: "BareOracleNoBits", Needs: api.NeedsTypeInfo, // bare Oracle filter w/o bits → excluded
			Oracle: &api.OracleFilter{Identifiers: []string{"!!"}}},
		{ID: "NarrowOracleFiltered", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle: &api.OracleFilter{Identifiers: []string{"!!"}}},
	}
	got := BuildOracleFilterRulesV2(rules, false)
	if len(got) != 3 {
		t.Fatalf("got %d rules, want 3 (only explicit oracle consumers pass through)", len(got))
	}

	byName := map[string]oracle.FilterRule{}
	for _, r := range got {
		byName[r.Name] = r
	}
	for _, want := range []string{"OracleAll", "OracleFiltered", "NarrowOracleFiltered"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("%s missing from filter set: %+v", want, byName)
		}
	}
	for _, skip := range []string{"SyntacticRule", "ResolverOnly", "TypeInfoOnly", "BareOracleNoBits"} {
		if _, leaked := byName[skip]; leaked {
			t.Errorf("non-oracle rule leaked through: %s", skip)
		}
	}

	for _, name := range []string{"OracleAll"} {
		r := byName[name]
		if r.Filter == nil || !r.Filter.AllFiles {
			t.Errorf("%s: Filter=%+v, want AllFiles:true default", name, r.Filter)
		}
	}
	wantIDs := map[string]string{"OracleFiltered": "suspend", "NarrowOracleFiltered": "!!"}
	for name, want := range wantIDs {
		r := byName[name]
		if r.Filter == nil || r.Filter.AllFiles ||
			len(r.Filter.Identifiers) != 1 || r.Filter.Identifiers[0] != want {
			t.Errorf("%s: Filter=%+v, want Identifiers:[%s]", name, r.Filter, want)
		}
	}
}

func TestBuildOracleFilterRulesV2_NotThoroughDropsThoroughFields(t *testing.T) {
	rules := []*api.Rule{
		{ID: "OracleThorough", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle: &api.OracleFilter{
				Identifiers:             []string{"base"},
				ThoroughOnlyIdentifiers: []string{"extra"},
				ThoroughOnlyAllFiles:    true,
			}},
	}
	got := BuildOracleFilterRulesV2(rules, false)
	if len(got) != 1 {
		t.Fatalf("got %d rules, want 1", len(got))
	}
	spec := got[0].Filter
	if spec.AllFiles {
		t.Errorf("ThoroughOnlyAllFiles should not promote AllFiles at balanced; got AllFiles=true")
	}
	if len(spec.Identifiers) != 1 || spec.Identifiers[0] != "base" {
		t.Errorf("ThoroughOnlyIdentifiers should not appear at balanced; got %v", spec.Identifiers)
	}
}

func TestBuildOracleFilterRulesV2_ThoroughMergesIdentifiers(t *testing.T) {
	rules := []*api.Rule{
		{ID: "OracleThorough", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle: &api.OracleFilter{
				Identifiers:             []string{"base"},
				ThoroughOnlyIdentifiers: []string{"extra1", "extra2"},
			}},
	}
	got := BuildOracleFilterRulesV2(rules, true)
	if len(got) != 1 {
		t.Fatalf("got %d rules, want 1", len(got))
	}
	spec := got[0].Filter
	want := []string{"base", "extra1", "extra2"}
	if len(spec.Identifiers) != len(want) {
		t.Fatalf("identifiers = %v; want %v", spec.Identifiers, want)
	}
	for i := range want {
		if spec.Identifiers[i] != want[i] {
			t.Errorf("identifiers[%d] = %q; want %q", i, spec.Identifiers[i], want[i])
		}
	}
}

func TestBuildOracleFilterRulesV2_ThoroughAllFilesPromotes(t *testing.T) {
	rules := []*api.Rule{
		{ID: "OracleThorough", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle: &api.OracleFilter{
				Identifiers:          []string{"base"},
				ThoroughOnlyAllFiles: true,
			}},
	}
	got := BuildOracleFilterRulesV2(rules, true)
	if len(got) != 1 || !got[0].Filter.AllFiles {
		t.Fatalf("ThoroughOnlyAllFiles must promote AllFiles=true at thorough; got %+v", got[0].Filter)
	}
}

// Source rule pointer must not gain identifiers across calls — Registry
// pointers are shared with the global rule set.
func TestBuildOracleFilterRulesV2_ThoroughDoesNotMutateInput(t *testing.T) {
	in := &api.OracleFilter{
		Identifiers:             []string{"base"},
		ThoroughOnlyIdentifiers: []string{"extra"},
	}
	rules := []*api.Rule{
		{ID: "OracleThorough", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets, Oracle: in},
	}
	_ = BuildOracleFilterRulesV2(rules, true)
	_ = BuildOracleFilterRulesV2(rules, true)
	if len(in.Identifiers) != 1 || in.Identifiers[0] != "base" {
		t.Fatalf("input rule's Identifiers grew across calls: %v", in.Identifiers)
	}
}

func TestBuildOracleFilterRulesV2_NoOracleRulesReturnsEmpty(t *testing.T) {
	rules := []*api.Rule{
		{ID: "A"},
		{ID: "B", Needs: api.NeedsResolver},
		{ID: "C", Needs: api.NeedsLinePass},
	}
	got := BuildOracleFilterRulesV2(rules, false)
	if len(got) != 0 {
		t.Errorf("got %d rules, want 0 — no rule declared NeedsOracle", len(got))
	}
}
