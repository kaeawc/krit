package rules

import (
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func findModuleAwareV2Rule(id string) *v2.Rule {
	for _, r := range v2.Registry {
		if r.ID == id {
			return r
		}
	}
	return nil
}

func TestCollectModuleAwareNeedsV2(t *testing.T) {
	t.Run("graph-only rules stay lightweight", func(t *testing.T) {
		var rs []*v2.Rule
		for _, id := range []string{"CompileSdkMismatchAcrossModules", "ConventionPluginDeadCode"} {
			r := findModuleAwareV2Rule(id)
			if r == nil {
				t.Skipf("rule %q not found in registry — skip", id)
			}
			rs = append(rs, r)
		}
		needs := CollectModuleAwareNeedsV2(rs)
		if needs.NeedsFiles || needs.NeedsDependencies || needs.NeedsIndex {
			t.Fatalf("expected graph-only rules to stay lightweight, got %+v", needs)
		}
	})

	t.Run("package-cycle only requests module files", func(t *testing.T) {
		r := findModuleAwareV2Rule("PackageDependencyCycle")
		if r == nil {
			t.Skip("PackageDependencyCycle not found in registry")
		}
		needs := CollectModuleAwareNeedsV2([]*v2.Rule{r})
		if !needs.NeedsFiles {
			t.Fatalf("expected module files to be required, got %+v", needs)
		}
		if needs.NeedsDependencies || needs.NeedsIndex {
			t.Fatalf("expected package cycle rule to avoid deps/index, got %+v", needs)
		}
	})

	t.Run("dead code keeps full module analysis", func(t *testing.T) {
		r := findModuleAwareV2Rule("ModuleDeadCode")
		if r == nil {
			t.Skip("ModuleDeadCode not found in registry")
		}
		needs := CollectModuleAwareNeedsV2([]*v2.Rule{r})
		if !needs.NeedsFiles || !needs.NeedsDependencies || !needs.NeedsIndex {
			t.Fatalf("expected dead code rule to require full module analysis, got %+v", needs)
		}
	})

	t.Run("unknown module-aware rules default conservative", func(t *testing.T) {
		r := findModuleAwareV2Rule("CompileSdkMismatchAcrossModules")
		if r == nil {
			t.Skip("CompileSdkMismatchAcrossModules not found in registry")
		}
		// A synthetic rule with no Implementation (no ModuleAwareRuleTuning) should default conservative.
		synthetic := &v2.Rule{ID: "SyntheticModuleAware", Needs: v2.NeedsModuleIndex}
		needs := CollectModuleAwareNeedsV2([]*v2.Rule{r, synthetic})
		if !needs.NeedsFiles || !needs.NeedsDependencies || !needs.NeedsIndex {
			t.Fatalf("expected unknown rule to preserve conservative behavior, got %+v", needs)
		}
	})
}

func TestDeadCodeCapabilityContracts(t *testing.T) {
	cases := []struct {
		id          string
		required    v2.Capabilities
		forbidden   v2.Capabilities
		description string
	}{
		{
			id:          "DeadCode",
			required:    v2.NeedsCrossFile,
			forbidden:   v2.NeedsModuleIndex | v2.NeedsParsedFiles | v2.NeedsResolver | v2.NeedsOracle,
			description: "cross-file index only",
		},
		{
			id:          "ModuleDeadCode",
			required:    v2.NeedsModuleIndex,
			forbidden:   v2.NeedsCrossFile | v2.NeedsParsedFiles | v2.NeedsResolver | v2.NeedsOracle,
			description: "module index only",
		},
		{
			id:          "UnsafeCallOnNullableType",
			required:    0,
			forbidden:   v2.NeedsCrossFile | v2.NeedsModuleIndex | v2.NeedsParsedFiles | v2.NeedsResolver | v2.NeedsTypeInfo | v2.NeedsOracle,
			description: "local AST/import evidence only",
		},
	}

	for _, tc := range cases {
		rule := findRegisteredRule(t, tc.id)
		if tc.required != 0 && !rule.Needs.Has(tc.required) {
			t.Fatalf("%s should require %s, got Needs=%b", tc.id, tc.description, rule.Needs)
		}
		if tc.required == 0 && rule.Needs != 0 {
			t.Fatalf("%s should stay %s; got Needs=%b", tc.id, tc.description, rule.Needs)
		}
		if rule.Needs&tc.forbidden != 0 {
			t.Fatalf("%s should stay %s; got forbidden Needs bits %b in Needs=%b", tc.id, tc.description, rule.Needs&tc.forbidden, rule.Needs)
		}
		if RuleNeedsKotlinOracle(rule) {
			t.Fatalf("%s should not contribute to KAA, got Oracle=%+v OracleCallTargets=%+v OracleDeclarationNeeds=%+v",
				tc.id, rule.Oracle, rule.OracleCallTargets, rule.OracleDeclarationNeeds)
		}
	}
}
