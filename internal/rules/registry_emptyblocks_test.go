package rules_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegisterEmptyblocksRules_AllRulesPresent guards against silently
// dropping a rule when registerEmptyblocksRules() splits its inlined
// blocks across per-rule helper functions.
func TestRegisterEmptyblocksRules_AllRulesPresent(t *testing.T) {
	expected := []string{
		"EmptyCatchBlock",
		"EmptyClassBlock",
		"EmptyDefaultConstructor",
		"EmptyDoWhileBlock",
		"EmptyElseBlock",
		"EmptyFinallyBlock",
		"EmptyForBlock",
		"EmptyFunctionBlock",
		"EmptyIfBlock",
		"EmptyInitBlock",
		"EmptyKotlinFile",
		"EmptySecondaryConstructor",
		"EmptyTryBlock",
		"EmptyWhenBlock",
		"EmptyWhileBlock",
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
			t.Errorf("expected empty-blocks rule %q to be registered", id)
			continue
		}
		if rule.Description == "" {
			t.Errorf("empty-blocks rule %q has empty Description", id)
		}
		if rule.Check == nil {
			t.Errorf("empty-blocks rule %q has no Check function", id)
		}
	}
}
