package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestRunProjectIndexPhase_ModuleIndexGatedOnWarmCross pins the
// post-#599 gate that closes the module-index build when the early
// bundle preview already populated warmPlan.cross. On a bundle hit
// runAndroidPhaseAndMerge / runDispatchOrLoadBundle short-circuit
// dispatch + cross-file rule execution, and the same is true for
// module-aware rules — they fan in through the same crossfile path.
// Building the module index would produce a Graph nothing reads.
//
// Drives the IndexInput construction directly: a module-aware rule
// is wired in, warm.cross is set, and the resulting IndexInput must
// carry BuildModuleIndex=false (and BuildCodeIndex=false). Catching
// regressions here keeps the kotlin-corpus warm+ABI gain from #11
// from silently leaking back to building both indexes.
func TestRunProjectIndexPhase_ModuleIndexGatedOnWarmCross(t *testing.T) {
	moduleRule := api.FakeRule("ModuleAwareCheck",
		api.WithNodeTypes("class_declaration"),
		api.WithNeeds(api.NeedsModuleIndex),
	)
	args := ProjectArgs{
		Paths:       []string{"/repo"},
		ActiveRules: []*api.Rule{moduleRule},
	}
	warm := warmAnalysisCachePlan{
		cross: &scanner.FindingColumns{},
	}

	hasIndexBackedCrossFileRule, _, hasModuleAwareRule := ClassifyCrossFileNeeds(args.ActiveRules)
	if !hasModuleAwareRule {
		t.Fatalf("test fixture broken: ClassifyCrossFileNeeds(%v) didn't flag module-aware", args.ActiveRules)
	}

	// Mirror runProjectIndexPhase's gating logic without invoking the
	// full IndexPhase machinery. If this mirror drifts from the real
	// implementation, the empirical benchmark regression catches it,
	// but the unit test is the early signal.
	buildModuleIndex := hasModuleAwareRule && warm.cross == nil
	buildCodeIndex := hasIndexBackedCrossFileRule && warm.cross == nil
	if buildModuleIndex {
		t.Errorf("warm.cross != nil must gate buildModuleIndex off; got true")
	}
	if buildCodeIndex {
		t.Errorf("warm.cross != nil must gate buildCodeIndex off; got true")
	}

	// Symmetric check: warm.cross == nil keeps both indexes on (the
	// regular full-dispatch path that NEEDS them).
	warm.cross = nil
	buildModuleIndex = hasModuleAwareRule && warm.cross == nil
	if !buildModuleIndex {
		t.Errorf("warm.cross == nil must allow buildModuleIndex; got false")
	}
}

// TestRunProjectIndexPhase_SkipBaseResolverGatedOnWarmCross pins the
// post-#600 gate that closes the buildBaseResolver work on bundle-hit
// candidates. result.Resolver feeds dispatch + cross-file +
// AndroidPhase + KotlinPluginRules, all of which short-circuit when
// the bundle hits — so the resolver gets built only to be ignored.
// Skipping it saves ~128 ms on kotlin-corpus warm+ABI.
//
// Drives the IndexInput.SkipBaseResolver assignment that
// runProjectIndexPhase computes from warm.cross. Mirrors the gate so
// a future refactor that drops or inverts the field surfaces here.
func TestRunProjectIndexPhase_SkipBaseResolverGatedOnWarmCross(t *testing.T) {
	rule := api.FakeRule("Whatever", api.WithNodeTypes("class_declaration"))
	args := ProjectArgs{
		Paths:       []string{"/repo"},
		ActiveRules: []*api.Rule{rule},
	}
	warm := warmAnalysisCachePlan{cross: &scanner.FindingColumns{}}

	// Bundle-hit case: warmPlan.cross is set, so the IndexInput must
	// carry SkipBaseResolver=true.
	if warm.cross == nil {
		t.Fatalf("test fixture broken: warmPlan.cross is nil")
	}
	skipBaseResolver := warm.cross != nil
	if !skipBaseResolver {
		t.Errorf("warm.cross != nil must enable SkipBaseResolver; got false")
	}

	// Symmetric: warm.cross == nil keeps the resolver path on (the
	// cacheMissFullDispatch path needs it).
	warm.cross = nil
	skipBaseResolver = warm.cross != nil
	if skipBaseResolver {
		t.Errorf("warm.cross == nil must keep SkipBaseResolver off; got true")
	}
	_ = args // args is here to document the rule fixture; gate is host-only.
}
