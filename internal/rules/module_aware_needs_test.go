package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

type conservativeModuleAwareRule struct{ BaseRule }

func (r *conservativeModuleAwareRule) Check(_ *scanner.File) []scanner.Finding { return nil }
func (r *conservativeModuleAwareRule) SetModuleIndex(_ *module.PerModuleIndex) {}
func (r *conservativeModuleAwareRule) CheckModuleAware() []scanner.Finding     { return nil }

func TestCollectModuleAwareNeeds(t *testing.T) {
	t.Run("graph-only rules stay lightweight", func(t *testing.T) {
		needs := CollectModuleAwareNeeds([]Rule{
			&CompileSdkMismatchAcrossModulesRule{},
			&ConventionPluginDeadCodeRule{},
		})
		if needs.NeedsFiles || needs.NeedsDependencies || needs.NeedsIndex {
			t.Fatalf("expected graph-only rules to stay lightweight, got %+v", needs)
		}
	})

	t.Run("package-cycle only requests module files", func(t *testing.T) {
		needs := CollectModuleAwareNeeds([]Rule{&PackageDependencyCycleRule{}})
		if !needs.NeedsFiles {
			t.Fatalf("expected module files to be required, got %+v", needs)
		}
		if needs.NeedsDependencies || needs.NeedsIndex {
			t.Fatalf("expected package cycle rule to avoid deps/index, got %+v", needs)
		}
	})

	t.Run("dead code keeps full module analysis", func(t *testing.T) {
		needs := CollectModuleAwareNeeds([]Rule{&ModuleDeadCodeRule{}})
		if !needs.NeedsFiles || !needs.NeedsDependencies || !needs.NeedsIndex {
			t.Fatalf("expected dead code rule to require full module analysis, got %+v", needs)
		}
	})

	t.Run("unknown module-aware rules default conservative", func(t *testing.T) {
		needs := CollectModuleAwareNeeds([]Rule{
			&CompileSdkMismatchAcrossModulesRule{},
			&conservativeModuleAwareRule{},
		})
		if !needs.NeedsFiles || !needs.NeedsDependencies || !needs.NeedsIndex {
			t.Fatalf("expected unknown rule to preserve conservative behavior, got %+v", needs)
		}
	})
}
