package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
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

func TestBuildOracleCallTargetFilterV2_Bounded(t *testing.T) {
	rules := []*v2.Rule{
		{ID: "SyntacticRule"},
		{ID: "NoCallTarget", Needs: v2.NeedsTypeInfo},
		{ID: "Suspend", Needs: v2.NeedsTypeInfo, OracleCallTargets: &v2.OracleCallTargetFilter{
			TargetFQNs:  []string{"kotlinx.coroutines.delay"},
			CalleeNames: []string{"await", "delay"},
		}},
		{ID: "Cast", Needs: v2.NeedsTypeInfo, OracleCallTargets: &v2.OracleCallTargetFilter{
			CalleeNames: []string{"getSystemService", "findViewById"},
		}},
	}

	got := BuildOracleCallTargetFilterV2(rules)
	if !got.Enabled {
		t.Fatalf("filter disabled: %+v", got)
	}
	want := []string{"await", "delay", "findViewById", "getSystemService"}
	if len(got.CalleeNames) != len(want) {
		t.Fatalf("callee names = %v, want %v", got.CalleeNames, want)
	}
	for i := range want {
		if got.CalleeNames[i] != want[i] {
			t.Fatalf("callee names = %v, want %v", got.CalleeNames, want)
		}
	}
	if got.Fingerprint == "" {
		t.Fatal("expected non-empty fingerprint")
	}
}

func TestBuildOracleCallTargetFilterV2_BroadDisables(t *testing.T) {
	rules := []*v2.Rule{
		{ID: "Bounded", Needs: v2.NeedsTypeInfo, OracleCallTargets: &v2.OracleCallTargetFilter{CalleeNames: []string{"delay"}}},
		{ID: "Broad", Needs: v2.NeedsTypeInfo, OracleCallTargets: &v2.OracleCallTargetFilter{AllCalls: true}},
	}

	got := BuildOracleCallTargetFilterV2(rules)
	if got.Enabled {
		t.Fatalf("filter enabled, want disabled: %+v", got)
	}
	if len(got.DisabledBy) != 1 || got.DisabledBy[0] != "Broad" {
		t.Fatalf("DisabledBy = %v, want [Broad]", got.DisabledBy)
	}
}

func TestBuildOracleCallTargetFilterV2_DerivesAnnotatedCallees(t *testing.T) {
	files := []*scanner.File{{
		Path: "src/main/kotlin/Api.kt",
		Content: []byte(`package test

import kotlin.Deprecated as Old

class Api {
    @Old("use fun replacement")
    fun oldCall() = Unit

    @get:CheckResult
    val mustUse: String = ""
}

@CheckReturnValue
fun topLevelMustUse(): String = ""
`),
	}}
	rules := []*v2.Rule{{
		ID:    "AnnotatedCalls",
		Needs: v2.NeedsTypeInfo,
		OracleCallTargets: &v2.OracleCallTargetFilter{
			AnnotatedIdentifiers: []string{"Deprecated", "CheckReturnValue", "CheckResult"},
		},
	}}

	got := BuildOracleCallTargetFilterV2ForFiles(rules, files)
	if !got.Enabled {
		t.Fatalf("filter disabled: %+v", got)
	}
	for _, want := range []string{"mustUse", "oldCall", "topLevelMustUse"} {
		if !containsString(got.CalleeNames, want) {
			t.Fatalf("callee names = %v, missing %q", got.CalleeNames, want)
		}
	}
	if got.Fingerprint == "" {
		t.Fatal("expected non-empty fingerprint")
	}
}

func TestBuildOracleCallTargetFilterV2_AnnotatedCalleesNeedFiles(t *testing.T) {
	rules := []*v2.Rule{{
		ID:    "AnnotatedCalls",
		Needs: v2.NeedsTypeInfo,
		OracleCallTargets: &v2.OracleCallTargetFilter{
			AnnotatedIdentifiers: []string{"Deprecated"},
		},
	}}

	got := BuildOracleCallTargetFilterV2(rules)
	if got.Enabled {
		t.Fatalf("filter enabled without files, want conservative disable: %+v", got)
	}
	if len(got.DisabledBy) != 1 || got.DisabledBy[0] != "AnnotatedCalls" {
		t.Fatalf("DisabledBy = %v, want [AnnotatedCalls]", got.DisabledBy)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
