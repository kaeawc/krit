package rules

import "testing"

func TestAllRulesHaveDescription(t *testing.T) {
	missing := 0
	for _, r := range Registry {
		if r.Description() == "" {
			t.Errorf("rule %s (%s) has no description", r.Name(), r.RuleSet())
			missing++
		}
	}
	if missing > 0 {
		t.Fatalf("%d rules missing descriptions out of %d total", missing, len(Registry))
	}
	t.Logf("all %d rules have descriptions", len(Registry))
}
