package rules_test

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestPrecompileConventions enforces docs/precompile/taxonomy.md
// against every registered precompile rule.
func TestPrecompileConventions(t *testing.T) {
	for _, r := range api.Registry {
		if r.Category != api.CategoryPrecompile {
			continue
		}
		r := r
		t.Run(r.ID, func(t *testing.T) {
			if r.Level == api.LevelUnset {
				t.Errorf("rule %s has no Level set; precompile rules must declare a RuleLevel", r.ID)
			}
			if r.Level == api.LevelMeta {
				if r.Sev != api.SeverityError && r.Sev != api.SeverityWarning {
					t.Errorf("meta rule %s severity %q must be error or warning", r.ID, r.Sev)
				}
			} else {
				if r.Sev != api.SeverityError {
					t.Errorf("rule %s severity %q must be %q for precompile category",
						r.ID, r.Sev, api.SeverityError)
				}
			}
			if r.DefaultActive {
				t.Errorf("rule %s declares DefaultActive=true; precompile rules must be opt-in (DefaultActive=false)",
					r.ID)
			}
			if r.Description == "" {
				t.Errorf("rule %s has empty Description", r.ID)
			}
		})
	}
}
