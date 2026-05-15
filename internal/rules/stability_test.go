package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestEveryDefaultActiveRuleHasStability is the contract gate from #196:
// every default-active rule must resolve to a non-Unset Stability so
// downstream consumers (baseline tooling, CI gates) can rely on the
// commitment without per-rule special cases. The default derivation lives
// in V2RuleStability, so this test covers both rules that declared
// Stability explicitly and rules that fall through to the default.
func TestEveryDefaultActiveRuleHasStability(t *testing.T) {
	missing := 0
	for _, r := range api.Registry {
		if !IsDefaultActive(r.ID) {
			continue
		}
		if V2RuleStability(r) == api.StabilityUnset {
			t.Errorf("default-active rule %s has no Stability tier", r.ID)
			missing++
		}
	}
	if missing > 0 {
		t.Fatalf("%d default-active rules missing Stability", missing)
	}
}

func TestV2RuleStabilityDefaults(t *testing.T) {
	cases := []struct {
		name string
		rule *api.Rule
		want api.Stability
	}{
		{
			name: "explicit override wins",
			rule: &api.Rule{ID: "X", Stability: api.StabilityFrozen},
			want: api.StabilityFrozen,
		},
		{
			name: "experimental defaults to evolving",
			rule: &api.Rule{ID: "X", Maturity: api.MaturityExperimental},
			want: api.StabilityEvolving,
		},
		{
			name: "stable rule defaults to stable",
			rule: &api.Rule{ID: "X"},
			want: api.StabilityStable,
		},
		{
			name: "nil rule treated as evolving",
			rule: nil,
			want: api.StabilityEvolving,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := V2RuleStability(c.rule); got != c.want {
				t.Fatalf("V2RuleStability = %v, want %v", got, c.want)
			}
		})
	}
}
