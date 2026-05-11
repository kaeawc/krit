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
			if !api.IsPrecompileID(r.ID) {
				t.Errorf("rule ID %q does not match precompile pattern %s", r.ID, api.PrecompileIDPattern)
			}
			if api.IsPrecompileMetaID(r.ID) {
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
