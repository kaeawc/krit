package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRulesNeedParsedSource_ModuleIndexForcesParsedSource is the
// regression guard for the warm-rerun module-aware finding drop. Before
// the fix, a rule that only declared NeedsModuleIndex took the
// cache-miss-only parse path on warm reruns; with zero misses
// parseResult.SourceFiles() was empty, the per-module ModuleFiles map
// was empty, and module-aware findings (ModuleDeadCode,
// PackageDependencyCycle, VersionCatalogUnused, ...) silently dropped
// to zero. The fix treats module-aware rules as needing parsed source
// so the warm path re-parses on rerun.
func TestRulesNeedParsedSource_ModuleIndexForcesParsedSource(t *testing.T) {
	moduleRule := api.FakeRule("ModuleAware", api.WithNeeds(api.NeedsModuleIndex))
	if !RulesNeedParsedSource([]*api.Rule{moduleRule}, true, true) {
		t.Fatal("NeedsModuleIndex rule must force parsed source on warm rerun; otherwise the per-module index sees an empty file set and emits zero findings")
	}
	cacheResult := &cache.Result{
		TotalCached: 1,
		TotalFiles:  1,
		CachedPaths: map[string]bool{"a.kt": true},
	}
	if CanParseOnlyCacheMisses([]*api.Rule{moduleRule}, cacheResult, true, true, true) {
		t.Fatal("CanParseOnlyCacheMisses must return false for NeedsModuleIndex rules")
	}
}
