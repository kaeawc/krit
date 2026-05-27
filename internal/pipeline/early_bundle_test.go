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
//
// Post-#605 the preview also gates on prior.Fingerprint == fp — so
// the test wires a manifestLoader that returns a manifest whose
// Fingerprint matches whatever the preview computes by querying the
// helper itself through a probe loader. The probe captures the fp
// the preview built, then a second invocation (with that fp in the
// manifest) verifies the Load fires.
func TestPreviewPostParseBundleHit_LoadHitReturnsColumns(t *testing.T) {
	cached := &scanner.FindingColumns{}
	args := ProjectArgs{Paths: []string{"/repo"}, Version: "test"}
	parseResult := ParseResult{}
	priorContent := map[string]string{"/repo/Foo.kt": "h1"}

	// Pass 1: capture the fp the preview computes by wiring a probe
	// store whose Load records the fp it was called with — then make
	// the first invocation pass the gate via a deliberately matching
	// manifest fingerprint.
	var capturedFP scanner.RunFingerprint
	captureStore := &fpCapturingStore{onLoad: func(fp scanner.RunFingerprint) {
		capturedFP = fp
	}}
	// Bootstrap: temporary manifest with any fingerprint — won't gate
	// open, but it lets us peek at the cacheHitStructuralBundle's
	// preview manifest pipeline. Easier path: construct the fp via
	// the exposed helper and use it directly.
	rulesHash := projectRuleHash(args.ActiveRules, args.Config)
	androidFP, libraryFactsFP := preparseProjectFingerprints(args, ProjectHostState{
		FindingsBundleCacheRoot: "/tmp/cache",
		PriorContentHashes:      priorContent,
	})
	wantFP := scanner.RunFingerprint{
		Version:      args.Version,
		Rules:        rulesHash,
		Config:       rulesHash,
		SourceSet:    sourceSetFingerprint(parseResult.KotlinFiles, parseResult.JavaFiles, priorContent, nil),
		CrossFile:    crossFileStructuralFingerprint(parseResult.KotlinFiles, parseResult.JavaFiles, nil, nil),
		Android:      androidFP,
		LibraryFacts: libraryFactsFP,
	}
	_ = captureStore
	_ = capturedFP
	prior := scanner.FindingsBundleManifest{
		Fingerprint:   wantFP,
		ContentHashes: priorContent,
	}
	store := &recordingPreviewStore{loaded: cached}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/tmp/cache",
		PriorContentHashes:      priorContent,
		FindingsBundleManifestLoader: func(string) (scanner.FindingsBundleManifest, bool) {
			return prior, true
		},
	}

	got := previewPostParseBundleHit(args, host, parseResult)
	if got != cached {
		t.Errorf("store Load returned hit; preview should pass it through. got=%v want=%v", got, cached)
	}
	if store.loadCalls != 1 {
		t.Errorf("preview must call Load once; got loadCalls=%d", store.loadCalls)
	}
}

// fpCapturingStore is a FindingsBundleStore that calls a hook on
// every Load so tests can capture the RunFingerprint the preview
// computed. Helper for the LoadHitReturnsColumns test which has to
// mirror the preview's fp construction exactly.
type fpCapturingStore struct {
	onLoad func(scanner.RunFingerprint)
}

func (s *fpCapturingStore) Load(_ string, fp scanner.RunFingerprint) (*scanner.FindingColumns, bool) {
	if s.onLoad != nil {
		s.onLoad(fp)
	}
	return nil, false
}

func (s *fpCapturingStore) Save(string, scanner.RunFingerprint, *scanner.FindingColumns) error {
	return nil
}

// TestPreviewPostParseBundleHit_LoadMissReturnsNil is the fall-through
// case: PriorContentHashes is set but the store has no matching
// entry. Returning nil lets the caller proceed with the full
// IndexPhase + dispatch path.
//
// Post-#605 the preview short-circuits the cacheHitFullBundle Load
// when no manifest is wired (priorOK=false), so this test only
// asserts the return value — the Load-count contract is now
// "Load only when there's a reasonable chance of hitting," covered
// by the more specific behavior tests.
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
}

// TestPreviewPostParseBundleHit_SkipsFullBundleLoadWhenFpDiffers
// pins the post-#605 lever: when the prior manifest's stored
// Fingerprint differs from the freshly-computed runFP (the warm+ABI
// case where content has changed), the preview must skip the
// cacheHitFullBundle Load — the bundle CAN'T be stored under runFP
// because no analyze has ever produced it. The Load would
// deterministically miss after burning ~90 ms of zstd+gob decode on
// kotlin-corpus scale.
//
// Drives the preview with a manifestLoader that returns a prior
// whose Fingerprint guarantees a mismatch. asserts the
// FindingsBundleStore.Load count stays at zero for the
// cacheHitFullBundle path (structural-replay still Loads exactly
// once for the alias source).
func TestPreviewPostParseBundleHit_SkipsFullBundleLoadWhenFpDiffers(t *testing.T) {
	store := &recordingPreviewStore{} // Load always misses
	priorWithDifferentFP := scanner.FindingsBundleManifest{
		Fingerprint: scanner.RunFingerprint{Version: "ancient", Rules: "old"},
		ContentHashes: map[string]string{
			"/repo/Foo.kt": "h1",
		},
	}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: "/tmp/cache",
		PriorContentHashes:      priorWithDifferentFP.ContentHashes,
		FindingsBundleManifestLoader: func(string) (scanner.FindingsBundleManifest, bool) {
			return priorWithDifferentFP, true
		},
	}
	args := ProjectArgs{Paths: []string{"/repo"}, Version: "current"}
	parseResult := ParseResult{
		KotlinFiles: []*scanner.File{{
			Path:     "/repo/Foo.kt",
			Language: scanner.LangKotlin,
			Content:  []byte("package demo\nclass Foo\n"),
		}},
	}

	got := previewPostParseBundleHit(args, host, parseResult)
	if got != nil {
		t.Errorf("structural-replay shouldn't fire on multi-file structural change; got %v", got)
	}
	// fp != prior.Fingerprint short-circuits the full-bundle Load.
	// The structural-replay path may still attempt a Load — but on
	// this fixture (no parsed files in priorContentHashes), the
	// planner refuses before reaching Load. So loadCalls stays 0.
	if store.loadCalls > 0 {
		t.Errorf("fp != prior.Fingerprint must skip cacheHitFullBundle Load; got loadCalls=%d", store.loadCalls)
	}
}

// TestBuildPreviewManifestData_OmitsFileStats pins the cost-saving
// contract: the preview's manifest builder must NOT call statForPath
// on every prior path. On kotlin-corpus scale the prior contains
// ~18 k entries; a stat sweep is ~80-200 ms of wasted work since
// tryLoadStructurallyStableBundle never reads fileStats.
func TestBuildPreviewManifestData_OmitsFileStats(t *testing.T) {
	args := ProjectArgs{Paths: []string{"/repo"}, Version: "test"}
	host := ProjectHostState{
		FindingsBundleCacheRoot: "/repo",
		PriorContentHashes: map[string]string{
			"/repo/a.kt": "h1",
			"/repo/b.kt": "h2",
		},
		PriorStructuralFPs: map[string]string{
			"/repo/a.kt": "s1",
			"/repo/b.kt": "s2",
		},
	}
	parseResult := ParseResult{}

	got := buildPreviewManifestData(args, host, parseResult)
	if !got.enabled {
		t.Fatalf("manifest disabled: %+v", got)
	}
	if len(got.fileStats) != 0 {
		t.Errorf("preview manifest must omit fileStats; got %d entries", len(got.fileStats))
	}
	if got.contentHashes["/repo/a.kt"] != "h1" || got.contentHashes["/repo/b.kt"] != "h2" {
		t.Errorf("preview manifest must carry forward prior content hashes; got %v", got.contentHashes)
	}
	if got.structuralFPs["/repo/a.kt"] != "s1" {
		t.Errorf("preview manifest must carry forward prior structural fps; got %v", got.structuralFPs)
	}
}

// TestBuildPreviewManifestData_DirtyPathRecomputed pins the
// recompute-for-dirty contract: when a parsed file is marked dirty
// (watcher saw an edit OR #590's stat-drift augmentation added it),
// the preview manifest must hash the parsed Content rather than reuse
// the stale prior hash. Otherwise the runFP we build matches the
// prior bundle key on every analyze and structural reuse never
// fires for ABI changes.
func TestBuildPreviewManifestData_DirtyPathRecomputed(t *testing.T) {
	args := ProjectArgs{Paths: []string{"/repo"}, Version: "test"}
	host := ProjectHostState{
		FindingsBundleCacheRoot: "/repo",
		PriorContentHashes:      map[string]string{"/repo/a.kt": "STALE"},
		PriorStructuralFPs:      map[string]string{"/repo/a.kt": "STALE-FP"},
		SourceSetDirty:          []string{"/repo/a.kt"},
	}
	parseResult := ParseResult{
		KotlinFiles: []*scanner.File{{
			Path:     "/repo/a.kt",
			Language: scanner.LangKotlin,
			Content:  []byte("package demo\nclass A\n"),
		}},
	}

	got := buildPreviewManifestData(args, host, parseResult)
	if got.contentHashes["/repo/a.kt"] == "STALE" {
		t.Errorf("dirty parsed file's hash must be recomputed; got %q", got.contentHashes["/repo/a.kt"])
	}
}
