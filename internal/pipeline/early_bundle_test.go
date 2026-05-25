package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// recordingPreviewStore counts Load and Save calls so the
// previewPostParseBundleHit tests can assert hit/miss behavior without
// a full RunProject fixture.
type recordingPreviewStore struct {
	loadCalls int
	loaded    *scanner.FindingColumns
	saved     map[string]*scanner.FindingColumns
}

func (s *recordingPreviewStore) Load(_ string, fp scanner.RunFingerprint) (*scanner.FindingColumns, bool) {
	s.loadCalls++
	if s.saved == nil {
		return s.loaded, s.loaded != nil
	}
	v, ok := s.saved[scanner.FindingsBundleKey(fp)]
	return v, ok
}

func (s *recordingPreviewStore) Save(_ string, fp scanner.RunFingerprint, cols *scanner.FindingColumns) error {
	if s.saved == nil {
		s.saved = make(map[string]*scanner.FindingColumns)
	}
	s.saved[scanner.FindingsBundleKey(fp)] = cols
	return nil
}

// TestPreviewPostParseBundleHit_SkipsLoadWhenNoPriorManifest pins the
// gate that protects the existing bundle-cache load-count contract:
// on cold runs the host has no PriorContentHashes (the manifest
// hasn't been persisted yet), so the preview can never hit by
// construction. Returning early avoids an extra Load that would
// otherwise show up in the bundle-cache test fixtures and break the
// "1 Load per analyze" invariant.
func TestPreviewPostParseBundleHit_SkipsLoadWhenNoPriorManifest(t *testing.T) {
	store := &recordingPreviewStore{}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/tmp/cache",
		// PriorContentHashes intentionally nil (cold run).
	}
	args := ProjectArgs{Version: "test"}
	parseResult := ParseResult{
		KotlinFiles: []*scanner.File{{Path: "/repo/Foo.kt", Language: scanner.LangKotlin}},
	}

	got := previewPostParseBundleHit(args, host, parseResult)
	if got != nil {
		t.Errorf("cold-run preview should return nil; got %v", got)
	}
	if store.loadCalls != 0 {
		t.Errorf("cold-run preview must not call Load; got loadCalls=%d", store.loadCalls)
	}
}

// TestPreviewPostParseBundleHit_NoStoreReturnsNil pins the CLI-path
// contract: callers that don't wire a FindingsBundleStore (CLI / tests
// that don't care about the bundle cache) get a nil return without
// any extra work. Mirrors computeRunFingerprint's gate.
func TestPreviewPostParseBundleHit_NoStoreReturnsNil(t *testing.T) {
	host := ProjectHostState{
		// FindingsBundleStore intentionally nil.
		PriorContentHashes: map[string]string{"/repo/Foo.kt": "h1"},
	}
	args := ProjectArgs{Version: "test"}
	parseResult := ParseResult{}

	got := previewPostParseBundleHit(args, host, parseResult)
	if got != nil {
		t.Errorf("nil store must return nil; got %v", got)
	}
}

// TestPreviewPostParseBundleHit_CustomRuleJarsBypass mirrors
// computeRunFingerprint's --custom-rule-jars gate: bundle caching
// disables when custom rule jars are loaded (output depends on
// jar-side behavior that the runFP doesn't capture). The preview
// must respect the same gate so it doesn't serve stale findings on a
// custom-rule-jars invocation.
func TestPreviewPostParseBundleHit_CustomRuleJarsBypass(t *testing.T) {
	store := &recordingPreviewStore{}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/tmp/cache",
		PriorContentHashes:      map[string]string{"/repo/Foo.kt": "h1"},
	}
	args := ProjectArgs{Version: "test", CustomRuleJars: []string{"/path/to/jar"}}
	parseResult := ParseResult{}

	got := previewPostParseBundleHit(args, host, parseResult)
	if got != nil {
		t.Errorf("custom-rule-jars path must return nil; got %v", got)
	}
	if store.loadCalls != 0 {
		t.Errorf("custom-rule-jars path must not call Load; got loadCalls=%d", store.loadCalls)
	}
}

// TestPreviewPostParseBundleHit_LoadHitReturnsColumns is the happy
// path: prior manifest present, store has a matching entry, the
// preview returns the cached FindingColumns so the caller can
// populate warmPlan.cross and skip IndexPhase's codeIndexBuild.
func TestPreviewPostParseBundleHit_LoadHitReturnsColumns(t *testing.T) {
	cached := &scanner.FindingColumns{}
	store := &recordingPreviewStore{loaded: cached}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/tmp/cache",
		PriorContentHashes:      map[string]string{"/repo/Foo.kt": "h1"},
	}
	args := ProjectArgs{Version: "test"}
	parseResult := ParseResult{}

	got := previewPostParseBundleHit(args, host, parseResult)
	if got != cached {
		t.Errorf("store Load returned hit; preview should pass it through. got=%v want=%v", got, cached)
	}
	if store.loadCalls != 1 {
		t.Errorf("preview must call Load once; got loadCalls=%d", store.loadCalls)
	}
}

// TestPreviewPostParseBundleHit_LoadMissReturnsNil is the fall-through
// case: PriorContentHashes is set but the store has no matching
// entry. Returning nil lets the caller proceed with the full
// IndexPhase + dispatch path.
func TestPreviewPostParseBundleHit_LoadMissReturnsNil(t *testing.T) {
	store := &recordingPreviewStore{} // no loaded, no saved
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/tmp/cache",
		PriorContentHashes:      map[string]string{"/repo/Foo.kt": "h1"},
	}
	args := ProjectArgs{Version: "test"}
	parseResult := ParseResult{}

	got := previewPostParseBundleHit(args, host, parseResult)
	if got != nil {
		t.Errorf("Load miss must return nil; got %v", got)
	}
	if store.loadCalls != 1 {
		t.Errorf("preview must still call Load once on miss; got loadCalls=%d", store.loadCalls)
	}
}
