package ruletest

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// withRegisteredRule installs a synthetic rule into api.Registry for the
// duration of the test, restoring the prior contents on cleanup.
func withRegisteredRule(t *testing.T, r *api.Rule) {
	t.Helper()
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })
	api.Registry = append(append([]*api.Rule{}, saved...), r)
}

// alwaysFireCheck is a Check that emits one finding on every node it
// receives. Combined with NodeTypes: nil + Needs = 0 it dispatches as a
// "every node" rule via ScopePerFileAllNodes, which the harness routes
// through the same code path real rules use.
func alwaysFireCheck(name string) func(*api.Context) {
	var fired atomic.Bool
	return func(ctx *api.Context) {
		if fired.Swap(true) {
			return
		}
		ctx.Emit(scanner.Finding{
			Rule:     name,
			Message:  "synthetic " + name + " finding",
			Line:     1,
			Col:      1,
			Severity: "warning",
		})
	}
}

func TestLintSource_AcceptsZeroNeedsRule(t *testing.T) {
	withRegisteredRule(t, &api.Rule{
		ID:          "TestRuletestLintSourceRule",
		Description: "synthetic zero-needs rule for ruletest happy path",
		Check:       alwaysFireCheck("TestRuletestLintSourceRule"),
	})
	got := LintSource(t, "TestRuletestLintSourceRule", "class A\n")
	if len(got) == 0 {
		t.Fatal("LintSource returned no findings; expected synthetic rule to fire")
	}
}

func TestLintSourceJava_AcceptsZeroNeedsRule(t *testing.T) {
	withRegisteredRule(t, &api.Rule{
		ID:          "TestRuletestLintSourceJavaRule",
		Description: "synthetic zero-needs rule for Java ruletest happy path",
		Languages:   []scanner.Language{scanner.LangJava},
		Check:       alwaysFireCheck("TestRuletestLintSourceJavaRule"),
	})
	got := LintSourceJava(t, "TestRuletestLintSourceJavaRule",
		"class Test { void m() {} }\n")
	if len(got) == 0 {
		t.Fatal("LintSourceJava returned no findings; expected synthetic rule to fire")
	}
}

func TestLintWithResolver_AcceptsNeedsResolver(t *testing.T) {
	withRegisteredRule(t, &api.Rule{
		ID:          "TestRuletestLintWithResolverRule",
		Description: "synthetic NeedsResolver rule for ruletest happy path",
		Needs:       api.NeedsResolver,
		Check:       alwaysFireCheck("TestRuletestLintWithResolverRule"),
	})
	got := LintWithResolver(t, "TestRuletestLintWithResolverRule", "class A\n")
	if len(got) == 0 {
		t.Fatal("LintWithResolver returned no findings; expected synthetic rule to fire")
	}
}

func TestLintWithFakeOracle_AcceptsNeedsOracle(t *testing.T) {
	withRegisteredRule(t, &api.Rule{
		ID:          "TestRuletestLintWithFakeOracleRule",
		Description: "synthetic NeedsOracle rule for ruletest happy path",
		Needs:       api.NeedsOracle | api.NeedsResolver,
		Check:       alwaysFireCheck("TestRuletestLintWithFakeOracleRule"),
	})
	fake := oracle.NewFakeOracle()
	got := LintWithFakeOracle(t, "TestRuletestLintWithFakeOracleRule", "class A\n", fake)
	if len(got) == 0 {
		t.Fatal("LintWithFakeOracle returned no findings; expected synthetic rule to fire")
	}
}

func TestLintWithFakeOracle_NilFakeUsesSourceResolverOnly(t *testing.T) {
	withRegisteredRule(t, &api.Rule{
		ID:          "TestRuletestLintWithFakeOracleNilRule",
		Description: "synthetic rule for nil-fake oracle path",
		Needs:       api.NeedsResolver,
		Check:       alwaysFireCheck("TestRuletestLintWithFakeOracleNilRule"),
	})
	got := LintWithFakeOracle(t, "TestRuletestLintWithFakeOracleNilRule", "class A\n", nil)
	if len(got) == 0 {
		t.Fatal("LintWithFakeOracle with nil fake returned no findings")
	}
}

// Tier-validation tests target validateTier directly. The exported
// helpers call t.Fatalf on a tier mismatch, but t.Fatalf inside a
// sub-test propagates failure to the parent — so we test the pure
// validation function instead and assume the helpers wire it correctly
// (verified by the happy-path tests above).

func TestValidateTier_LintSourceTier_RejectsNeedsResolver(t *testing.T) {
	rule := &api.Rule{ID: "TestRuletestValidateResolver", Needs: api.NeedsResolver}
	err := validateTier(rule, "LintSource", 0)
	if err == nil {
		t.Fatal("validateTier accepted NeedsResolver for the LintSource tier")
	}
	if !strings.Contains(err.Error(), "higher-tier helper") {
		t.Errorf("expected tier-mismatch hint in error, got: %v", err)
	}
}

func TestValidateTier_AnyTier_RejectsProjectScopeCapability(t *testing.T) {
	cases := []api.Capabilities{
		api.NeedsCrossFile,
		api.NeedsModuleIndex,
		api.NeedsParsedFiles,
		api.NeedsManifest,
		api.NeedsResources,
		api.NeedsGradle,
	}
	for _, cap := range cases {
		rule := &api.Rule{ID: "TestRuletestValidateProjectScope", Needs: cap}
		err := validateTier(rule, "LintWithFakeOracle", api.NeedsResolver|api.NeedsOracle)
		if err == nil {
			t.Errorf("validateTier accepted project-scope capability %v", cap)
			continue
		}
		if !strings.Contains(err.Error(), "integration harness") {
			t.Errorf("expected integration-harness hint for %v, got: %v", cap, err)
		}
	}
}

func TestValidateTier_LintWithResolverTier_RejectsNeedsOracle(t *testing.T) {
	rule := &api.Rule{ID: "TestRuletestValidateOracle", Needs: api.NeedsResolver | api.NeedsOracle}
	err := validateTier(rule, "LintWithResolver", api.NeedsResolver)
	if err == nil {
		t.Fatal("validateTier accepted NeedsOracle for the LintWithResolver tier")
	}
}

func TestValidateTier_AllowsScopeShapeBits(t *testing.T) {
	// NeedsLinePass / NeedsAggregate / NeedsConcurrent describe the
	// rule's dispatcher shape, not external context, so every tier
	// must accept them.
	for _, cap := range []api.Capabilities{api.NeedsLinePass, api.NeedsAggregate, api.NeedsConcurrent} {
		rule := &api.Rule{ID: "TestRuletestValidateShape", Needs: cap}
		if err := validateTier(rule, "LintSource", 0); err != nil {
			t.Errorf("validateTier rejected scope-shape bit %v at LintSource tier: %v", cap, err)
		}
	}
}

func TestValidateTier_AcceptsExpectedTierCapability(t *testing.T) {
	rule := &api.Rule{ID: "TestRuletestValidateExpected", Needs: api.NeedsResolver}
	if err := validateTier(rule, "LintWithResolver", api.NeedsResolver); err != nil {
		t.Errorf("validateTier rejected NeedsResolver at LintWithResolver tier: %v", err)
	}
}

func TestLookupRule_UnknownReturnsErrRuleNotRegistered(t *testing.T) {
	_, err := lookupRule("TestRuletestNoSuchRule_xyz")
	if !errors.Is(err, ErrRuleNotRegistered) {
		t.Errorf("lookupRule for unknown ID returned %v, want ErrRuleNotRegistered", err)
	}
}

func TestLookupRule_RegisteredReturnsRule(t *testing.T) {
	withRegisteredRule(t, &api.Rule{
		ID:          "TestRuletestLookupHit",
		Description: "synthetic rule for lookup test",
		Check:       func(*api.Context) {},
	})
	rule, err := lookupRule("TestRuletestLookupHit")
	if err != nil {
		t.Fatalf("lookupRule unexpected error: %v", err)
	}
	if rule == nil || rule.ID != "TestRuletestLookupHit" {
		t.Errorf("lookupRule returned %v, want rule with ID TestRuletestLookupHit", rule)
	}
}
