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
		// A synthetic rule with no OriginalV1 (no ModuleAwareRuleTuning) should default conservative.
		synthetic := &v2.Rule{ID: "SyntheticModuleAware", Needs: v2.NeedsModuleIndex}
		needs := CollectModuleAwareNeedsV2([]*v2.Rule{r, synthetic})
		if !needs.NeedsFiles || !needs.NeedsDependencies || !needs.NeedsIndex {
			t.Fatalf("expected unknown rule to preserve conservative behavior, got %+v", needs)
		}
	})
}
