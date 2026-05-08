package rules_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegisterAndroidCorrectnessRules_AllRulesPresent asserts that every
// rule registered through the previously inlined
// registerAndroidCorrectnessRules() god-function is still present after
// the per-rule split. The Description and Check fields are sanity-checked
// so a rule cannot be silently dropped or registered with a no-op body.
func TestRegisterAndroidCorrectnessRules_AllRulesPresent(t *testing.T) {
	expected := []string{
		"DefaultLocale",
		"CommitPrefEdits",
		"CommitTransaction",
		"Assert",
		"CheckResult",
		"ShiftFlags",
		"UniqueConstants",
		"WrongThread",
		"SQLiteString",
		"Registered",
		"NestedScrolling",
		"ScrollViewCount",
		"SimpleDateFormat",
		"SetTextI18n",
		"StopShip",
		"WrongCall",
	}

	registered := make(map[string]*api.Rule, len(expected))
	for _, r := range api.Registry {
		for _, id := range expected {
			if r.ID == id {
				registered[id] = r
				break
			}
		}
	}

	for _, id := range expected {
		rule, ok := registered[id]
		if !ok {
			t.Errorf("expected android-correctness rule %q to be registered", id)
			continue
		}
		if rule.Description == "" {
			t.Errorf("android-correctness rule %q has empty Description", id)
		}
		if rule.Check == nil {
			t.Errorf("android-correctness rule %q has no Check function", id)
		}
	}
}
