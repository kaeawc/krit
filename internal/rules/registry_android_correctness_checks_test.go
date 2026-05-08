package rules_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegisterAndroidCorrectnessChecksRules_AllRulesPresent guards against
// silently dropping a rule when registerAndroidCorrectnessChecksRules()
// splits its inlined blocks across per-rule helper functions.
func TestRegisterAndroidCorrectnessChecksRules_AllRulesPresent(t *testing.T) {
	expected := []string{
		"OverrideAbstract",
		"ParcelCreator",
		"SwitchIntDef",
		"TextViewEdits",
		"WrongViewCast",
		"Deprecated",
		"Range",
		"ResourceType",
		"ResourceAsColor",
		"SupportAnnotationUsage",
		"AccidentalOctal",
		"AppCompatMethod",
		"CustomViewStyleable",
		"InnerclassSeparator",
		"ObjectAnimatorBinding",
		"OnClick",
		"PropertyEscape",
		"ShortAlarm",
		"LocalSuppress",
		"PluralsCandidate",
	}
	want := make(map[string]struct{}, len(expected))
	for _, id := range expected {
		want[id] = struct{}{}
	}
	registered := make(map[string]*api.Rule, len(expected))
	for _, r := range api.Registry {
		if _, ok := want[r.ID]; ok {
			registered[r.ID] = r
		}
	}
	for _, id := range expected {
		rule, ok := registered[id]
		if !ok {
			t.Errorf("expected android-correctness-checks rule %q to be registered", id)
			continue
		}
		if rule.Description == "" {
			t.Errorf("android-correctness-checks rule %q has empty Description", id)
		}
		if rule.Check == nil {
			t.Errorf("android-correctness-checks rule %q has no Check function", id)
		}
	}
}
