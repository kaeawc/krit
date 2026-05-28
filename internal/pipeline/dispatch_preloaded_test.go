package pipeline

import (
	"context"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// loadCountingStore is a FindingsBundleStore that counts how many
// times Load fires. Used by the preload-bundle test to assert the
// short-circuit fired (loadCalls=0 when WarmCrossFindings is set).
type loadCountingBundleStore struct {
	loadCalls int
	saveCalls int
}

func (s *loadCountingBundleStore) Load(string, scanner.RunFingerprint) (*scanner.FindingColumns, bool) {
	s.loadCalls++
	return nil, false
}

func (s *loadCountingBundleStore) Save(string, scanner.RunFingerprint, *scanner.FindingColumns) error {
	s.saveCalls++
	return nil
}

// TestRunDispatchOrLoadBundle_PreloadedSkipsStoreLoad pins the
// short-circuit added on top of #603: when previewPostParseBundleHit
// already loaded the bundle and stashed it on indexResult.WarmCrossFindings,
// runDispatchOrLoadBundle must return it directly without calling
// FindingsBundleStore.Load again. Each Load is ~90ms of zstd+gob
// decode on kotlin-corpus scale; warm+ABI used to pay three of them
// (preview-full-miss + preview-prior-hit + dispatch-Load). The
// short-circuit eliminates the third.
func TestRunDispatchOrLoadBundle_PreloadedSkipsStoreLoad(t *testing.T) {
	preloaded := &scanner.FindingColumns{}
	store := &loadCountingBundleStore{}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/repo",
	}
	indexResult := IndexResult{
		WarmCrossFindings:          preloaded,
		WarmCrossFindingsAreBundle: true, // preview-bundle source
	}

	d, c, hit, _, err := runDispatchOrLoadBundle(
		context.Background(),
		ProjectArgs{},
		host,
		indexResult,
		ParseResult{},
		scanner.RunFingerprint{Version: "v1"},
		true, // bundleEnabled
		deltaManifestData{},
	)
	if err != nil {
		t.Fatalf("runDispatchOrLoadBundle: %v", err)
	}
	if !hit {
		t.Fatalf("expected bundle hit; got hit=false")
	}
	if store.loadCalls != 0 {
		t.Errorf("preloaded bundle must skip Load; got loadCalls=%d", store.loadCalls)
	}
	if d.Findings.Len() != preloaded.Len() {
		t.Errorf("DispatchResult.Findings did not carry the preloaded columns")
	}
	if c.Findings.Len() != preloaded.Len() {
		t.Errorf("CrossFileResult.Findings did not carry the preloaded columns")
	}
}

// TestRunDispatchOrLoadBundle_NoPreloadHitsStoreLoad confirms the
// fallback path is unchanged when WarmCrossFindings is nil — the
// usual cacheHitFullBundle / cacheHitStructuralBundle / dispatch
// flow must still drive FindingsBundleStore.Load.
func TestRunDispatchOrLoadBundle_NoPreloadHitsStoreLoad(t *testing.T) {
	store := &loadCountingBundleStore{}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/repo",
	}
	indexResult := IndexResult{
		// WarmCrossFindings intentionally nil.
	}

	_, _, _, _, _ = runDispatchOrLoadBundle(
		context.Background(),
		ProjectArgs{},
		host,
		indexResult,
		ParseResult{},
		scanner.RunFingerprint{Version: "v1"},
		true,
		deltaManifestData{},
	)
	if store.loadCalls < 1 {
		t.Errorf("nil WarmCrossFindings must drive at least one Load; got loadCalls=%d", store.loadCalls)
	}
}

// TestRunDispatchOrLoadBundle_AnalysisCacheCrossFindingsDoNotShortCircuit
// is the regression guard for the warm+ABI = 0 findings bug
// (#605's short-circuit treated WarmCrossFindings as the full
// bundle, but the lexically-irrelevant analysis-cache fallback only
// populates it with cross-file findings; short-circuiting there
// drops every per-file finding).
//
// The fix gates the short-circuit on WarmCrossFindingsAreBundle.
// Test wires a scenario where WarmCrossFindings is non-nil but the
// flag is false (= analysis-cache source) and asserts the
// short-circuit does NOT fire — the regular bundle layers run, and
// since the store has no matching entry, the dispatch falls through
// to the full-dispatch path that produces correct per-file findings.
func TestRunDispatchOrLoadBundle_AnalysisCacheCrossFindingsDoNotShortCircuit(t *testing.T) {
	crossOnly := &scanner.FindingColumns{}
	store := &loadCountingBundleStore{}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/repo",
	}
	indexResult := IndexResult{
		WarmCrossFindings: crossOnly,
		// WarmCrossFindingsAreBundle intentionally false — this is
		// the lexically-irrelevant analysis-cache fallback source,
		// which holds only cross-file findings.
		WarmCrossFindingsAreBundle: false,
	}

	_, _, hit, _, _ := runDispatchOrLoadBundle(
		context.Background(),
		ProjectArgs{},
		host,
		indexResult,
		ParseResult{},
		scanner.RunFingerprint{Version: "v1"},
		true,
		deltaManifestData{},
	)
	if hit {
		t.Errorf("non-bundle WarmCrossFindings must NOT short-circuit; got hit=true (would emit cross-only findings and drop per-file)")
	}
	if store.loadCalls < 1 {
		t.Errorf("non-bundle WarmCrossFindings must let the regular dispatchBundleLoad fire; got loadCalls=%d", store.loadCalls)
	}
}
