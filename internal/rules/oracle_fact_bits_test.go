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
			name: "Narrow CallTargets bit on Needs",
			rule: &api.Rule{
				ID:                "T",
				Needs:             api.NeedsOracleCallTargets,
				OracleCallTargets: &api.OracleCallTargetFilter{CalleeNames: []string{"x"}},
			},
			wantAny:  api.NeedsOracleCallTargets,
			wantNone: api.NeedsOracleDiagnostics | api.NeedsOracleLibraryClasses | api.NeedsOracleMembers,
		},
		{
			name: "Supertypes bit on Needs",
			rule: &api.Rule{
				ID:    "T",
				Needs: api.NeedsOracleSupertypes,
			},
			wantAny:  api.NeedsOracleSupertypes,
			wantNone: api.NeedsOracleMembers | api.NeedsOracleDiagnostics,
		},
		{
			name: "MemberAnnotations bit",
			rule: &api.Rule{
				ID:    "T",
				Needs: api.NeedsOracleMembers | api.NeedsOracleMemberAnnotations,
			},
			wantAny:  api.NeedsOracleMembers | api.NeedsOracleMemberAnnotations,
			wantNone: api.NeedsOracleDiagnostics,
		},
		{
			name: "LibraryClasses bit",
			rule: &api.Rule{
				ID:    "T",
				Needs: api.NeedsOracleLibraryClasses,
			},
			wantAny: api.NeedsOracleLibraryClasses,
		},
		{
			name: "Bare Oracle filter without bits contributes nothing",
			rule: &api.Rule{
				ID:     "T",
				Oracle: &api.OracleFilter{Identifiers: []string{"suspend"}},
			},
			wantNone: api.NeedsOracle,
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

// TestOracleFactUnion_NoLegacyIDLift documents that the rule-ID switch
// for diagnostic-consuming rules has been removed. The matching rules
// must declare NeedsOracleDiagnostics on Needs to opt in.
func TestOracleFactUnion_NoLegacyIDLift(t *testing.T) {
	for _, id := range []string{"UnsafeCast"} {
		r := &api.Rule{ID: id}
		got := OracleFactUnion([]*api.Rule{r})
		if got.HasAny(api.NeedsOracleDiagnostics) {
			t.Errorf("rule %q without bits should not contribute diagnostics, got %b", id, got)
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
		ID:    "DiagnosticOnlyRule",
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

// TestOracleBitsMatchMetadata fails if any registered rule has oracle
// metadata (OracleCallTargets or non-empty OracleDeclarationNeeds) but
// has not declared the matching NeedsOracle* bits. The bits are now
// the single source of truth for the JVM workload union — leaving
// metadata without bits silently downgrades the rule.
func TestOracleBitsMatchMetadata(t *testing.T) {
	for _, r := range api.Registry {
		if r == nil {
			continue
		}
		want := bitsImpliedByMetadata(r)
		got := r.Needs & api.NeedsOracle
		if missing := want & ^got; missing != 0 {
			t.Errorf("rule %q has oracle metadata implying %b but declared %b — missing bits %b", r.ID, want, got, missing)
		}
	}
}

// bitsImpliedByMetadata derives the oracle bits a rule's legacy
// metadata semantically asserts. Used by the live-registry guard to
// catch rules whose Needs declaration drifted from their metadata.
func bitsImpliedByMetadata(r *api.Rule) api.Capabilities {
	if r == nil {
		return 0
	}
	var bits api.Capabilities
	if r.OracleCallTargets != nil {
		bits |= api.NeedsOracleCallTargets
	}
	if n := r.OracleDeclarationNeeds; n != nil {
		if n.Supertypes {
			bits |= api.NeedsOracleSupertypes
		}
		if n.ClassAnnotations {
			bits |= api.NeedsOracleClassAnnotations
		}
		if n.Members {
			bits |= api.NeedsOracleMembers
		}
		if n.MemberSignatures {
			bits |= api.NeedsOracleMembers | api.NeedsOracleMemberSignatures
		}
		if n.MemberAnnotations {
			bits |= api.NeedsOracleMembers | api.NeedsOracleMemberAnnotations
		}
		if n.SourceDependencyClosure {
			bits |= api.NeedsOracleLibraryClasses
		}
	}
	return bits
}

// TestNoUmbrellaInLiveRegistry asserts no registered rule declares the
// legacy NeedsOracle umbrella (i.e. every narrow bit at once). New
// rules must declare narrow bits matching the facts their Check
// function actually consumes.
func TestNoUmbrellaInLiveRegistry(t *testing.T) {
	for _, r := range api.Registry {
		if r == nil {
			continue
		}
		if r.Needs.Has(api.NeedsOracle) {
			t.Errorf("rule %q declares the umbrella NeedsOracle — declare narrow NeedsOracle* bits matching the rule's actual KAA fact consumption", r.ID)
		}
	}
}
