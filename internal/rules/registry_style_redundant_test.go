package rules_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegisterStyleRedundantRules_AllRulesPresent guards against silently
// dropping a rule when registerStyleRedundantRules() splits its inlined
// blocks across per-rule helper functions.
func TestRegisterStyleRedundantRules_AllRulesPresent(t *testing.T) {
	expected := []string{
		"RedundantVisibilityModifier",
		"RedundantConstructorKeyword",
		"RedundantExplicitType",
		"UnnecessaryParentheses",
		"UnnecessaryInheritance",
		"UnnecessaryInnerClass",
		"OptionalUnit",
		"UnnecessaryBackticks",
		"UselessCallOnNotNull",
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
			t.Errorf("expected style-redundant rule %q to be registered", id)
			continue
		}
		if rule.Description == "" {
			t.Errorf("style-redundant rule %q has empty Description", id)
		}
		if rule.Check == nil {
			t.Errorf("style-redundant rule %q has no Check function", id)
		}
	}
}
