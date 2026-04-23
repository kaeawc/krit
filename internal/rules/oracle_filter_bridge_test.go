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
			LexicalHintsByCallee: map[string][]string{
				"await": {"kotlinx.coroutines"},
			},
			LexicalSkipByCallee: map[string][]string{
				"w": {"Log"},
			},
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
	if hints := got.LexicalHintsByCallee["await"]; len(hints) != 1 || hints[0] != "kotlinx.coroutines" {
		t.Fatalf("await lexical hints = %v, want [kotlinx.coroutines]", hints)
	}
	if hints := got.LexicalHintsByCallee["delay"]; len(hints) == 0 {
		t.Fatalf("delay lexical hints missing derived TargetFQN evidence: %+v", got.LexicalHintsByCallee)
	}
	if skips := got.LexicalSkipByCallee["w"]; len(skips) != 1 || skips[0] != "Log" {
		t.Fatalf("w lexical skips = %v, want [Log]", skips)
	}
	if len(got.RuleProfiles) != 2 {
		t.Fatalf("rule profiles = %+v, want 2", got.RuleProfiles)
	}
	if got.RuleProfiles[0].RuleID != "Cast" || got.RuleProfiles[1].RuleID != "Suspend" {
		t.Fatalf("rule profiles not sorted by rule ID: %+v", got.RuleProfiles)
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
	if len(got.RuleProfiles) != 2 || got.RuleProfiles[1].RuleID != "Broad" || got.RuleProfiles[1].DisabledReason != "allCalls" {
		t.Fatalf("RuleProfiles = %+v, want allCalls disabled profile", got.RuleProfiles)
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
	if len(got.RuleProfiles) != 1 || !containsString(got.RuleProfiles[0].DerivedCalleeNames, "oldCall") {
		t.Fatalf("RuleProfiles = %+v, want derived oldCall attribution", got.RuleProfiles)
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

func TestBuildOracleDeclarationProfileV2_FallsBackWhenUnoptedIn(t *testing.T) {
	// A rule with nil OracleDeclarationNeeds forces full profile.
	rules := []*v2.Rule{
		{ID: "Narrow", Needs: v2.NeedsTypeInfo,
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true}},
		{ID: "NotOptedIn", Needs: v2.NeedsTypeInfo}, // nil → conservative
	}
	got := BuildOracleDeclarationProfileV2(rules)
	if got.Fingerprint != "" {
		t.Fatalf("expected full-profile (empty fingerprint) when any rule has nil OracleDeclarationNeeds, got %q", got.Fingerprint)
	}
	if !got.Profile.IsFull() {
		t.Fatalf("expected full profile, got %+v", got.Profile)
	}
}

func TestBuildOracleDeclarationProfileV2_UnionWhenAllOptedIn(t *testing.T) {
	// All rules opted in — profile is the union of declared needs.
	rules := []*v2.Rule{
		{ID: "ExprOnly", Needs: v2.NeedsTypeInfo,
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{}},
		{ID: "NeedsSupertypes", Needs: v2.NeedsTypeInfo,
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true, Supertypes: true}},
		{ID: "NeedsAnnotations", Needs: v2.NeedsTypeInfo,
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{Members: true, MemberAnnotations: true}},
	}
	got := BuildOracleDeclarationProfileV2(rules)
	if got.Fingerprint == "" {
		t.Fatalf("all rules opted in with narrow profiles — fingerprint must be non-empty (not full profile)")
	}
	p := got.Profile
	if !p.ClassShell || !p.Supertypes || !p.Members || !p.MemberAnnotations {
		t.Fatalf("union must include ClassShell+Supertypes+Members+MemberAnnotations, got %+v", p)
	}
	if p.MemberSignatures || p.ClassAnnotations || p.SourceDependencyClosure {
		t.Fatalf("union must NOT include MemberSignatures/ClassAnnotations/SourceDependencyClosure, got %+v", p)
	}
}

func TestBuildOracleDeclarationProfileV2_SkipsNonOracleRules(t *testing.T) {
	// Non-oracle rules (NeedsResolver only) are ignored even with nil declaration needs.
	rules := []*v2.Rule{
		{ID: "ResolverOnly", Needs: v2.NeedsResolver}, // no NeedsOracle → ignored
		{ID: "OracleRule", Needs: v2.NeedsTypeInfo,
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true}},
	}
	got := BuildOracleDeclarationProfileV2(rules)
	if got.Fingerprint == "" {
		t.Fatalf("non-oracle rule must not force full profile; expected narrow fingerprint, got empty")
	}
}

func TestBuildOracleDeclarationProfileV2_EmptyRulesYieldsNarrow(t *testing.T) {
	// No oracle rules → all-opted-in vacuously, union is zero profile (all false).
	// Zero profile is narrower than full — fingerprint is non-empty.
	got := BuildOracleDeclarationProfileV2(nil)
	// With zero active oracle rules, allOptedIn=true and union is the zero
	// DeclarationProfile. FinalizeDeclarationProfile of a zero value is non-full,
	// so fingerprint is non-empty.
	if got.Fingerprint == "" {
		t.Fatalf("empty rule set: all-false profile must produce non-empty fingerprint (not full)")
	}
}

func TestBuildOracleDeclarationProfileV2_LiveRuleSet(t *testing.T) {
	// Verify that all registered NeedsOracle rules have opted in to narrowing
	// and that the resulting profile is non-full (skips MemberSignatures etc.).
	all := v2.Registry
	var oracleRules []*v2.Rule
	var notOptedIn []string
	for _, r := range all {
		if r == nil || !r.Needs.Has(v2.NeedsOracle) {
			continue
		}
		oracleRules = append(oracleRules, r)
		if r.OracleDeclarationNeeds == nil {
			notOptedIn = append(notOptedIn, r.ID)
		}
	}
	if len(notOptedIn) > 0 {
		t.Errorf("rules with NeedsOracle but nil OracleDeclarationNeeds (force full profile): %v", notOptedIn)
	}
	if len(oracleRules) == 0 {
		t.Skip("no oracle rules registered")
	}
	summary := BuildOracleDeclarationProfileV2(oracleRules)
	if summary.Profile.IsFull() {
		t.Errorf("all rules opted in but profile is still full — union logic or profile fields are wrong")
	}
	if summary.Fingerprint == "" {
		t.Errorf("narrow profile must produce non-empty fingerprint")
	}
	// Verify that MemberSignatures is NOT in the union (no current rule needs it).
	if summary.Profile.MemberSignatures {
		t.Errorf("MemberSignatures must not be in the union of current oracle rules")
	}
	// Verify that SourceDependencyClosure is NOT in the union.
	if summary.Profile.SourceDependencyClosure {
		t.Errorf("SourceDependencyClosure must not be in the union of current oracle rules")
	}
	t.Logf("live profile fingerprint: %q", summary.Fingerprint)
	t.Logf("live profile: %+v", summary.Profile)
}
