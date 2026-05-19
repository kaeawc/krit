package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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

func TestBuildOracleCallTargetFilterV2_Bounded(t *testing.T) {
	rules := []*api.Rule{
		{ID: "SyntacticRule"},
		{ID: "NoCallTarget", Needs: api.NeedsTypeInfo},
		{ID: "Suspend", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets, OracleCallTargets: &api.OracleCallTargetFilter{
			TargetFQNs:  []string{"kotlinx.coroutines.delay"},
			CalleeNames: []string{"await", "delay"},
			LexicalHintsByCallee: map[string][]string{
				"await": {"kotlinx.coroutines"},
			},
			LexicalSkipByCallee: map[string][]string{
				"w": {"Log"},
			},
		}},
		{ID: "Cast", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets, OracleCallTargets: &api.OracleCallTargetFilter{
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
	rules := []*api.Rule{
		{ID: "Bounded", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets, OracleCallTargets: &api.OracleCallTargetFilter{CalleeNames: []string{"delay"}}},
		{ID: "Broad", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets, OracleCallTargets: &api.OracleCallTargetFilter{AllCalls: true}},
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
	rules := []*api.Rule{{
		ID:    "AnnotatedCalls",
		Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
		OracleCallTargets: &api.OracleCallTargetFilter{
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
	rules := []*api.Rule{{
		ID:    "AnnotatedCalls",
		Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
		OracleCallTargets: &api.OracleCallTargetFilter{
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
	// A rule that declares the legacy umbrella NeedsOracle forces the
	// full profile because we cannot tell which fields it reads.
	rules := []*api.Rule{
		{ID: "Narrow", Needs: api.NeedsTypeInfo | api.NeedsOracleSupertypes,
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{ClassShell: true}},
		{ID: "NotOptedIn", Needs: api.NeedsOracle}, // umbrella → conservative
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
	rules := []*api.Rule{
		{ID: "ExprOnly", Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{}},
		{ID: "NeedsSupertypes", Needs: api.NeedsTypeInfo | api.NeedsOracleSupertypes,
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{ClassShell: true, Supertypes: true}},
		{ID: "NeedsAnnotations", Needs: api.NeedsTypeInfo | api.NeedsOracleMembers | api.NeedsOracleMemberAnnotations,
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{Members: true, MemberAnnotations: true}},
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
	rules := []*api.Rule{
		{ID: "ResolverOnly", Needs: api.NeedsResolver}, // no NeedsOracle → ignored
		{ID: "OracleRule", Needs: api.NeedsTypeInfo | api.NeedsOracleSupertypes,
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{ClassShell: true}},
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
	// Verify that all registered oracle-consuming rules have opted in to narrowing
	// and that the resulting profile is non-full (skips MemberSignatures etc.).
	all := api.Registry
	var oracleRules []*api.Rule
	var notOptedIn []string
	for _, r := range all {
		if !RuleNeedsKotlinOracle(r) {
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
