package rules_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegisterNamingRules_AllRulesPresent guards against silently dropping
// a rule when registerNamingRules() splits its inlined blocks across
// per-rule helper functions.
func TestRegisterNamingRules_AllRulesPresent(t *testing.T) {
	expected := []string{
		"ClassNaming",
		"FunctionNaming",
		"VariableNaming",
		"PackageNaming",
		"EnumNaming",
		"BooleanPropertyNaming",
		"ConstructorParameterNaming",
		"ForbiddenClassName",
		"FunctionNameMaxLength",
		"FunctionNameMinLength",
		"FunctionParameterNaming",
		"InvalidPackageDeclaration",
		"LambdaParameterNaming",
		"MatchingDeclarationName",
		"MemberNameEqualsClassName",
		"NoNameShadowing",
		"NonBooleanPropertyPrefixedWithIs",
		"ObjectPropertyNaming",
		"TopLevelPropertyNaming",
		"VariableMaxLength",
		"VariableMinLength",
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
			t.Errorf("expected naming rule %q to be registered", id)
			continue
		}
		if rule.Description == "" {
			t.Errorf("naming rule %q has empty Description", id)
		}
		if rule.Check == nil {
			t.Errorf("naming rule %q has no Check function", id)
		}
	}
}
