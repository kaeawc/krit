package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestAllRulesHaveDescription(t *testing.T) {
	missing := 0
	for _, r := range api.Registry {
		if r.Description == "" {
			t.Errorf("rule %s (%s) has no description", r.ID, r.Category)
			missing++
		}
	}
	if missing > 0 {
		t.Fatalf("%d rules missing descriptions out of %d total", missing, len(api.Registry))
	}
	t.Logf("all %d rules have descriptions", len(api.Registry))
}
