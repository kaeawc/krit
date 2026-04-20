package rules

import (
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func TestAllRulesHaveDescription(t *testing.T) {
	missing := 0
	for _, r := range v2.Registry {
		if r.Description == "" {
			t.Errorf("rule %s (%s) has no description", r.ID, r.Category)
			missing++
		}
	}
	if missing > 0 {
		t.Fatalf("%d rules missing descriptions out of %d total", missing, len(v2.Registry))
	}
	t.Logf("all %d rules have descriptions", len(v2.Registry))
}
