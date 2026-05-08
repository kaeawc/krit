package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestOracleFactUnion_LegacyUmbrellaSetsAllBits(t *testing.T) {
	rules := []*api.Rule{{ID: "Legacy", Needs: api.NeedsOracle}}
	union := OracleFactUnion(rules)
	if !union.Has(api.NeedsOracle) {
		t.Fatalf("legacy NeedsOracle umbrella must contribute every narrow bit, got %b", union)
	}
	for _, bit := range []api.Capabilities{
		api.NeedsOracleCallTargets,
		api.NeedsOracleSuspendMarkers,
		api.NeedsOracleExprType,
		api.NeedsOracleExprAnnotations,
		api.NeedsOracleSupertypes,
		api.NeedsOracleMembers,
		api.NeedsOracleMemberSignatures,
		api.NeedsOracleClassAnnotations,
		api.NeedsOracleMemberAnnotations,
		api.NeedsOracleDiagnostics,
		api.NeedsOracleLibraryClasses,
	} {
		if !union.Has(bit) {
			t.Errorf("legacy umbrella must include bit %b", bit)
		}
	}
}

func TestOracleFactUnion_NarrowBitsContributeOnlyThemselves(t *testing.T) {
	rules := []*api.Rule{
		{ID: "Suspend", Needs: api.NeedsOracleCallTargets | api.NeedsOracleSuspendMarkers},
	}
	union := OracleFactUnion(rules)
	if !union.HasAny(api.NeedsOracleCallTargets) || !union.HasAny(api.NeedsOracleSuspendMarkers) {
		t.Fatalf("union missing declared bits: %b", union)
	}
	if union.HasAny(api.NeedsOracleDiagnostics) {
		t.Fatalf("union must not include undeclared NeedsOracleDiagnostics: %b", union)
	}
	if union.HasAny(api.NeedsOracleLibraryClasses) {
		t.Fatalf("union must not include undeclared NeedsOracleLibraryClasses: %b", union)
	}
	if union.HasAny(api.NeedsOracleMembers) {
		t.Fatalf("union must not include undeclared NeedsOracleMembers: %b", union)
	}
}

func TestOracleFactUnion_LegacyMetadataLifted(t *testing.T) {
	cases := []struct {
		name     string
		rule     *api.Rule
		wantAny  api.Capabilities
		wantNone api.Capabilities
	}{
		{
			name: "OracleCallTargets implies NeedsOracleCallTargets",
			rule: &api.Rule{
				ID:                "T",
				OracleCallTargets: &api.OracleCallTargetFilter{CalleeNames: []string{"x"}},
			},
			wantAny:  api.NeedsOracleCallTargets,
			wantNone: api.NeedsOracleDiagnostics | api.NeedsOracleLibraryClasses | api.NeedsOracleMembers,
		},
		{
			name: "OracleDeclarationNeeds.Supertypes lifts to bit",
			rule: &api.Rule{
				ID:                     "T",
				OracleDeclarationNeeds: &api.OracleDeclarationProfile{Supertypes: true},
			},
			wantAny:  api.NeedsOracleSupertypes,
			wantNone: api.NeedsOracleMembers | api.NeedsOracleDiagnostics,
		},
		{
			name: "OracleDeclarationNeeds.MemberAnnotations implies Members + MemberAnnotations",
			rule: &api.Rule{
				ID:                     "T",
				OracleDeclarationNeeds: &api.OracleDeclarationProfile{MemberAnnotations: true},
			},
			wantAny:  api.NeedsOracleMembers | api.NeedsOracleMemberAnnotations,
			wantNone: api.NeedsOracleDiagnostics,
		},
		{
			name: "OracleDeclarationNeeds.SourceDependencyClosure lifts to LibraryClasses",
			rule: &api.Rule{
				ID:                     "T",
				OracleDeclarationNeeds: &api.OracleDeclarationProfile{SourceDependencyClosure: true},
			},
			wantAny: api.NeedsOracleLibraryClasses,
		},
		{
			name: "Empty OracleDeclarationNeeds contributes no bits",
			rule: &api.Rule{
				ID:                     "T",
				OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			},
			wantNone: api.NeedsOracle,
		},
		{
			name: "Bare Oracle filter falls back to umbrella",
			rule: &api.Rule{
				ID:     "T",
				Oracle: &api.OracleFilter{Identifiers: []string{"suspend"}},
			},
			wantAny: api.NeedsOracle,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OracleFactUnion([]*api.Rule{tc.rule})
			if tc.wantAny != 0 && got&tc.wantAny != tc.wantAny {
				t.Errorf("missing wantAny=%b in union=%b", tc.wantAny, got)
			}
			if tc.wantNone != 0 && got&tc.wantNone != 0 {
				t.Errorf("union %b leaked bits %b (forbidden)", got, got&tc.wantNone)
			}
		})
	}
}

func TestOracleFactUnion_DiagnosticRuleIDsLiftToBit(t *testing.T) {
	for _, id := range []string{"UnsafeCast", "UnreachableCode"} {
		r := &api.Rule{ID: id}
		got := OracleFactUnion([]*api.Rule{r})
		if !got.HasAny(api.NeedsOracleDiagnostics) {
			t.Errorf("rule %q: expected NeedsOracleDiagnostics in union, got %b", id, got)
		}
	}
}

func TestNeedsOracleDiagnostics_DrivenByBit(t *testing.T) {
	cases := []struct {
		name string
		rule *api.Rule
		want bool
	}{
		{
			name: "narrow diag bit",
			rule: &api.Rule{ID: "X", Needs: api.NeedsOracleDiagnostics},
			want: true,
		},
		{
			name: "umbrella includes diag",
			rule: &api.Rule{ID: "X", Needs: api.NeedsOracle},
			want: true,
		},
		{
			name: "call-targets only",
			rule: &api.Rule{ID: "X", Needs: api.NeedsOracleCallTargets},
			want: false,
		},
		{
			name: "no oracle",
			rule: &api.Rule{ID: "X", Needs: api.NeedsResolver},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NeedsOracleDiagnostics([]*api.Rule{tc.rule})
			if got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestNeedsOracleLibraryClasses_DrivenByBit(t *testing.T) {
	cases := []struct {
		name string
		rule *api.Rule
		want bool
	}{
		{"narrow lib bit", &api.Rule{ID: "X", Needs: api.NeedsOracleLibraryClasses}, true},
		{"umbrella", &api.Rule{ID: "X", Needs: api.NeedsOracle}, true},
		{"legacy SourceDependencyClosure", &api.Rule{ID: "X", OracleDeclarationNeeds: &api.OracleDeclarationProfile{SourceDependencyClosure: true}}, true},
		{"call-targets only", &api.Rule{ID: "X", Needs: api.NeedsOracleCallTargets}, false},
		{"no oracle", &api.Rule{ID: "X"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NeedsOracleLibraryClasses([]*api.Rule{tc.rule})
			if got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestNeedsOracleDeclarationWalk_DrivenByBit(t *testing.T) {
	cases := []struct {
		name string
		rule *api.Rule
		want bool
	}{
		{"supertypes bit", &api.Rule{ID: "X", Needs: api.NeedsOracleSupertypes}, true},
		{"members bit", &api.Rule{ID: "X", Needs: api.NeedsOracleMembers}, true},
		{"member sigs bit", &api.Rule{ID: "X", Needs: api.NeedsOracleMemberSignatures}, true},
		{"class annotations bit", &api.Rule{ID: "X", Needs: api.NeedsOracleClassAnnotations}, true},
		{"member annotations bit", &api.Rule{ID: "X", Needs: api.NeedsOracleMemberAnnotations}, true},
		{"call-targets only", &api.Rule{ID: "X", Needs: api.NeedsOracleCallTargets}, false},
		{"diagnostics only", &api.Rule{ID: "X", Needs: api.NeedsOracleDiagnostics}, false},
		{"library only", &api.Rule{ID: "X", Needs: api.NeedsOracleLibraryClasses}, false},
		{"legacy decl needs", &api.Rule{ID: "X", OracleDeclarationNeeds: &api.OracleDeclarationProfile{Members: true}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NeedsOracleDeclarationWalk([]*api.Rule{tc.rule})
			if got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestBuildOracleDeclarationProfileV2_NarrowBitsDoNotForceFull(t *testing.T) {
	// A rule that declares only NeedsOracleCallTargets (no
	// OracleDeclarationNeeds) must NOT force the full profile — its
	// bits do not imply any declaration walk.
	rules := []*api.Rule{
		{ID: "CallTargetOnly", Needs: api.NeedsOracleCallTargets},
	}
	got := BuildOracleDeclarationProfileV2(rules)
	if got.Profile.IsFull() {
		t.Fatalf("call-target-only rule must not force full profile, got %+v", got.Profile)
	}
	if got.Profile.Members || got.Profile.Supertypes || got.Profile.MemberAnnotations {
		t.Fatalf("call-target-only rule must not request declaration fields, got %+v", got.Profile)
	}
}

func TestBuildOracleDeclarationProfileV2_BitsImplyProfileFields(t *testing.T) {
	// NeedsOracleSupertypes implies ClassShell + Supertypes;
	// NeedsOracleMemberSignatures implies ClassShell + Members + MemberSignatures.
	rules := []*api.Rule{
		{ID: "A", Needs: api.NeedsOracleSupertypes},
		{ID: "B", Needs: api.NeedsOracleMemberSignatures},
	}
	got := BuildOracleDeclarationProfileV2(rules).Profile
	if !got.ClassShell {
		t.Errorf("supertypes/member sigs bits must imply ClassShell")
	}
	if !got.Supertypes {
		t.Errorf("expected Supertypes")
	}
	if !got.Members {
		t.Errorf("MemberSignatures bit must imply Members")
	}
	if !got.MemberSignatures {
		t.Errorf("expected MemberSignatures")
	}
	if got.MemberAnnotations || got.ClassAnnotations {
		t.Errorf("undeclared annotation bits leaked: %+v", got)
	}
}

func TestRuleNeedsKotlinOracle_NarrowBitsCount(t *testing.T) {
	cases := []struct {
		name string
		rule *api.Rule
		want bool
	}{
		{"narrow call-targets bit", &api.Rule{ID: "X", Needs: api.NeedsOracleCallTargets}, true},
		{"narrow diagnostics bit", &api.Rule{ID: "X", Needs: api.NeedsOracleDiagnostics}, true},
		{"resolver only", &api.Rule{ID: "X", Needs: api.NeedsResolver}, false},
		{"empty", &api.Rule{ID: "X"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RuleNeedsKotlinOracle(tc.rule); got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestCapabilities_HasAny(t *testing.T) {
	c := api.NeedsOracleCallTargets | api.NeedsOracleSuspendMarkers
	if !c.HasAny(api.NeedsOracle) {
		t.Errorf("HasAny(NeedsOracle) must match a single narrow bit")
	}
	if c.Has(api.NeedsOracle) {
		t.Errorf("Has(NeedsOracle) must require every narrow bit")
	}
	if c.HasAny(api.NeedsOracleDiagnostics) {
		t.Errorf("HasAny must return false when bit is absent")
	}
}

// TestOracleNarrowing_EndToEnd pins the proposal's headline workload
// shrinkage: when only narrow oracle rules are active, the resulting
// invocation gates (declaration profile, diagnostics, library walk)
// must all narrow to "skip what no rule asked for".
func TestOracleNarrowing_EndToEnd(t *testing.T) {
	// Suspend-only rule (RedundantSuspendModifier shape).
	suspendOnly := &api.Rule{
		ID:    "SuspendOnly",
		Needs: api.NeedsOracleCallTargets | api.NeedsOracleSuspendMarkers,
		OracleCallTargets: &api.OracleCallTargetFilter{
			CalleeNames: []string{"delay", "yield"},
		},
		OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
	}

	// Cast rule (WrongViewCast shape) needs supertypes.
	castRule := &api.Rule{
		ID: "ViewCast",
		Needs: api.NeedsOracleCallTargets |
			api.NeedsOracleSupertypes |
			api.NeedsOracleLibraryClasses,
		OracleCallTargets: &api.OracleCallTargetFilter{
			CalleeNames: []string{"findViewById"},
		},
		OracleDeclarationNeeds: &api.OracleDeclarationProfile{
			ClassShell: true,
			Supertypes: true,
		},
	}

	// Diagnostic-only rule.
	diagOnly := &api.Rule{
		ID:    "UnreachableCode",
		Needs: api.NeedsOracleDiagnostics,
	}

	t.Run("suspend-only narrows everything", func(t *testing.T) {
		rules := []*api.Rule{suspendOnly}
		profile := BuildOracleDeclarationProfileV2(rules)
		if profile.Profile.IsFull() {
			t.Errorf("suspend-only rule must not force full profile, got %+v", profile.Profile)
		}
		if profile.Profile.Members || profile.Profile.Supertypes ||
			profile.Profile.MemberAnnotations || profile.Profile.ClassAnnotations ||
			profile.Profile.MemberSignatures || profile.Profile.SourceDependencyClosure {
			t.Errorf("suspend-only rule contributes no declaration fields, got %+v", profile.Profile)
		}
		if NeedsOracleDiagnostics(rules) {
			t.Errorf("suspend-only must not request diagnostics")
		}
		if NeedsOracleLibraryClasses(rules) {
			t.Errorf("suspend-only must not request library classes")
		}
		if NeedsOracleDeclarationWalk(rules) {
			t.Errorf("suspend-only must not request declaration walk")
		}
	})

	t.Run("cast rule walks supertypes + JAR but not members", func(t *testing.T) {
		rules := []*api.Rule{castRule}
		profile := BuildOracleDeclarationProfileV2(rules).Profile
		if !profile.Supertypes {
			t.Errorf("cast rule wants supertypes")
		}
		if !profile.SourceDependencyClosure {
			t.Errorf("cast rule wants library closure (JAR walk)")
		}
		if profile.Members || profile.MemberAnnotations || profile.MemberSignatures {
			t.Errorf("cast rule must not request member walks, got %+v", profile)
		}
		if NeedsOracleDiagnostics(rules) {
			t.Errorf("cast rule must not request diagnostics")
		}
		if !NeedsOracleLibraryClasses(rules) {
			t.Errorf("cast rule must request library classes")
		}
	})

	t.Run("diagnostic-only rule narrows declarations to none", func(t *testing.T) {
		rules := []*api.Rule{diagOnly}
		profile := BuildOracleDeclarationProfileV2(rules).Profile
		if profile.IsFull() {
			t.Errorf("diagnostic-only rule must not force full profile")
		}
		if profile.Members || profile.Supertypes || profile.SourceDependencyClosure {
			t.Errorf("diagnostic-only rule must not request declaration fields, got %+v", profile)
		}
		if !NeedsOracleDiagnostics(rules) {
			t.Errorf("diagnostic-only rule must request diagnostics")
		}
		if NeedsOracleLibraryClasses(rules) {
			t.Errorf("diagnostic-only rule must not request library classes")
		}
		if NeedsOracleDeclarationWalk(rules) {
			t.Errorf("diagnostic-only rule must not request declaration walk")
		}
	})

	t.Run("mixed set unions correctly", func(t *testing.T) {
		rules := []*api.Rule{suspendOnly, castRule, diagOnly}
		profile := BuildOracleDeclarationProfileV2(rules).Profile
		if !profile.Supertypes {
			t.Errorf("mixed set inherits supertypes from cast rule")
		}
		if profile.Members || profile.MemberAnnotations {
			t.Errorf("mixed set still skips member walks, got %+v", profile)
		}
		if !NeedsOracleDiagnostics(rules) {
			t.Errorf("mixed set requests diagnostics from diag rule")
		}
		if !NeedsOracleLibraryClasses(rules) {
			t.Errorf("mixed set requests library walk from cast rule")
		}
	})
}

// TestRuleNeedsKotlinOracle_LiveRegistry surfaces rules the bridge
// considers oracle-needing but that contribute zero fact bits — i.e.
// rules whose only oracle metadata is an empty OracleDeclarationProfile
// with no narrowing. These are migration leftovers; the non-empty list
// is logged so Phase 3 can clean them up. The test is intentionally
// non-fatal during the migration window.
func TestRuleNeedsKotlinOracle_LiveRegistry(t *testing.T) {
	var emptyMarkerOnly []string
	for _, r := range api.Registry {
		if !RuleNeedsKotlinOracle(r) {
			continue
		}
		if ruleOracleFactBits(r) == 0 {
			emptyMarkerOnly = append(emptyMarkerOnly, r.ID)
		}
	}
	if len(emptyMarkerOnly) > 0 {
		t.Logf("rules with empty OracleDeclarationNeeds and no other oracle interest (Phase 3 cleanup target): %v", emptyMarkerOnly)
	}
}
