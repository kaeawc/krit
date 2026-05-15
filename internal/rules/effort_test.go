package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// stubEffortProvider exercises the EffortProvider fallback path.
type stubEffortProvider struct{ e api.Effort }

func (s stubEffortProvider) Effort() api.Effort { return s.e }

func TestEffortDerivation(t *testing.T) {
	cases := []struct {
		name string
		rule *api.Rule
		want api.Effort
	}{
		{
			name: "explicit override wins",
			rule: &api.Rule{
				ID: "X", Category: "c", Description: "d", Sev: api.SeverityWarning,
				Needs: api.NeedsCrossFile, Effort: api.EffortTrivial,
				Check: func(*api.Context) {},
			},
			want: api.EffortTrivial,
		},
		{
			name: "provider fallback",
			rule: &api.Rule{
				ID: "X", Category: "c", Description: "d", Sev: api.SeverityWarning,
				NodeTypes:      []string{"call_expression"},
				Implementation: stubEffortProvider{e: api.EffortArchitectural},
				Check:          func(*api.Context) {},
			},
			want: api.EffortArchitectural,
		},
		{
			name: "cross-file rule derives refactor",
			rule: &api.Rule{
				ID: "X", Category: "c", Description: "d", Sev: api.SeverityWarning,
				Needs: api.NeedsCrossFile, Check: func(*api.Context) {},
			},
			want: api.EffortRefactor,
		},
		{
			name: "manifest rule derives refactor",
			rule: &api.Rule{
				ID: "X", Category: "c", Description: "d", Sev: api.SeverityWarning,
				Needs: api.NeedsManifest, Check: func(*api.Context) {},
			},
			want: api.EffortRefactor,
		},
		{
			name: "policy precision derives architectural",
			rule: &api.Rule{
				ID: "X", Category: "c", Description: "d", Sev: api.SeverityWarning,
				NodeTypes: []string{"call_expression"},
				Precision: api.PrecisionPolicy,
				Check:     func(*api.Context) {},
			},
			want: api.EffortArchitectural,
		},
		{
			name: "cosmetic-fix rule derives trivial",
			rule: &api.Rule{
				ID: "X", Category: "c", Description: "d", Sev: api.SeverityWarning,
				NodeTypes: []string{"call_expression"},
				Fix:       api.FixCosmetic,
				Check:     func(*api.Context) {},
			},
			want: api.EffortTrivial,
		},
		{
			name: "default per-file rule is local",
			rule: &api.Rule{
				ID: "X", Category: "c", Description: "d", Sev: api.SeverityWarning,
				NodeTypes: []string{"call_expression"}, Check: func(*api.Context) {},
			},
			want: api.EffortLocal,
		},
		{
			name: "nil rule returns local",
			rule: nil,
			want: api.EffortLocal,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := V2RuleEffort(c.rule); got != c.want {
				t.Errorf("V2RuleEffort = %v, want %v", got, c.want)
			}
		})
	}
}

// TestMetaForRule_EffortAlwaysSet verifies the no-zero-leakage invariant
// from the issue's evaluation criteria: every default-active (and every
// registered) rule reports a non-zero Effort via MetaForRule.
func TestMetaForRule_EffortAlwaysSet(t *testing.T) {
	for _, r := range api.Registry {
		desc, ok := MetaForRule(r)
		if !ok {
			t.Fatalf("MetaForRule(%s) returned ok=false", r.ID)
		}
		if desc.Effort == api.EffortUnset {
			t.Fatalf("rule %s: descriptor effort is EffortUnset", r.ID)
		}
	}
}

// TestMetaForRule_EffortExplicit verifies that an explicit Rule.Effort
// is mirrored on the descriptor without re-deriving.
func TestMetaForRule_EffortExplicit(t *testing.T) {
	r := &api.Rule{
		ID: "ExplicitEffortRule", Category: "test", Description: "explicit",
		Sev: api.SeverityWarning, NodeTypes: []string{"call_expression"},
		Effort: api.EffortArchitectural, Check: func(*api.Context) {},
	}
	desc, ok := MetaForRule(r)
	if !ok {
		t.Fatal("MetaForRule returned ok=false")
	}
	if desc.Effort != api.EffortArchitectural {
		t.Fatalf("effort = %v, want EffortArchitectural", desc.Effort)
	}
}
