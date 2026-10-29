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
				ID:    "T",
				Needs: api.NeedsOracleCallTargets,
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
