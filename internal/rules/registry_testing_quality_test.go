package rules_test

import (
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegisterTestingQualityRules_AllRulesPresent guards against silently
// dropping a rule when registerTestingQualityRules() splits its inlined
// blocks across per-rule helper functions.
func TestRegisterTestingQualityRules_AllRulesPresent(t *testing.T) {
	expected := []string{
		"AssertEqualsArgumentOrder",
		"AssertTrueOnComparison",
		"MixedAssertionLibraries",
		"AssertNullableWithNotNullAssertion",
		"MockWithoutVerify",
		"RunTestWithDelay",
		"RunTestWithThreadSleep",
		"RunBlockingInTest",
		"TestDispatcherNotInjected",
		"TestWithoutAssertion",
		"TestWithOnlyTodo",
		"TestFunctionReturnValue",
		"TestNameContainsUnderscore",
		"SharedMutableStateInObject",
		"TestInheritanceDepth",
		"UntestedPublicApi",
		"RelaxedMockUsedForValueClass",
		"SpyOnDataClass",
		"VerifyWithoutMock",
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
			t.Errorf("expected testing-quality rule %q to be registered", id)
			continue
		}
		if rule.Description == "" {
			t.Errorf("testing-quality rule %q has empty Description", id)
		}
		if rule.Check == nil {
			t.Errorf("testing-quality rule %q has no Check function", id)
		}
	}
}
