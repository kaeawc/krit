package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestRunProject_PhaseTimings_PopulatedOnFullRun asserts the wire
// format: ProjectResult.PhaseTimingsMs carries non-negative values
// for every phase that actually ran. On a cold cache-less run the
// parse + index phases are always > 0; dispatch / crossfile depend
// on whether any rule produces work but should at least be
// recorded (>=0).
func TestRunProject_PhaseTimings_PopulatedOnFullRun(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Foo.kt"),
		[]byte("package demo\n\nclass Foo : Any()\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	rule := findV2RuleForTest(t, "UnnecessaryInheritance")

	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{root},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	pt := res.PhaseTimingsMs
	if pt.Parse < 0 || pt.Index < 0 || pt.Dispatch < 0 ||
		pt.CrossFile < 0 || pt.Android < 0 || pt.Fixup < 0 || pt.Output < 0 {
		t.Errorf("phase timings must be non-negative: %+v", pt)
	}
	// Single-file fixtures can complete every phase in under a
	// millisecond and round to 0; the scale-side test asserts non-zero
	// behaviour. The contract here is just "all fields are populated
	// and non-negative".
}

// TestRunProject_PhaseTimings_BundleHitSkipsDispatchAndCrossfile is
// the diagnostic for warm-cache investigations: on a findings-bundle
// hit the dispatch and crossfile phases are bypassed entirely, so
// their wall-time must be 0. A non-zero reading after a bundle hit
// would indicate the helper still calls those phases — exactly the
// regression that observability is meant to catch.
func TestRunProject_PhaseTimings_BundleHitSkipsDispatchAndCrossfile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Foo.kt"),
		[]byte("package demo\n\nclass Foo\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	rule := findV2RuleForTest(t, "UnnecessaryInheritance")
	pc, err := scanner.NewParseCacheWithCap(root, -1)
	if err != nil {
		t.Fatalf("ParseCache: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	host := ProjectHostState{
		FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
		FindingsBundleCacheRoot: root,
		ParseCache:              pc,
	}
	args := ProjectArgs{
		Config:      config.NewConfig(),
		Paths:       []string{root},
		ActiveRules: []*api.Rule{rule},
		Format:      "json",
		Version:     "test",
	}

	first, err := RunProject(context.Background(), ProjectInput{Args: args, Host: host})
	if err != nil {
		t.Fatalf("first RunProject: %v", err)
	}
	if first.FindingsBundleHit {
		t.Fatalf("first call must miss the bundle; got hit=true")
	}

	second, err := RunProject(context.Background(), ProjectInput{Args: args, Host: host})
	if err != nil {
		t.Fatalf("second RunProject: %v", err)
	}
	if !second.FindingsBundleHit {
		t.Fatalf("second identical call must hit the bundle; got hit=false")
	}
	if second.PhaseTimingsMs.Dispatch != 0 {
		t.Errorf("bundle hit must skip dispatch (ms=0); got %d", second.PhaseTimingsMs.Dispatch)
	}
	if second.PhaseTimingsMs.CrossFile != 0 {
		t.Errorf("bundle hit must skip crossfile (ms=0); got %d", second.PhaseTimingsMs.CrossFile)
	}
}

// TestRunProject_PhaseTimings_BundleHitAtSyntheticScale is the
// scale-side companion to TestAnalyzeProject_BundleHitOnIdentical
// SecondCall in the serve package: the 2-file fixture proved the
// bundle key is stable for trivial inputs, but #139 points at a
// kotlin-corpus regression at thousands-of-files scale. This test
// runs the same contract against ~200 files to catch fingerprint
// drift that only surfaces at moderate scale.
//
// 200 is the upper bound at which this test stays under ~1 second
// in CI. If the bundle ever stops firing at this scale, the test
// reproduces the corpus regression without needing the actual
// JetBrains/kotlin checkout.
func TestRunProject_PhaseTimings_BundleHitAtSyntheticScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skip synthetic-scale bundle hit test in -short mode")
	}
	root := t.TempDir()
	const fileCount = 200
	for i := 0; i < fileCount; i++ {
		path := filepath.Join(root, fmt.Sprintf("F%03d.kt", i))
		body := fmt.Sprintf("package demo\n\nclass F%03d {\n    fun a%d() {}\n}\n", i, i)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	rule := findV2RuleForTest(t, "UnnecessaryInheritance")
	host := ProjectHostState{
		FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
		FindingsBundleCacheRoot: root,
	}
	args := ProjectArgs{
		Config:      config.NewConfig(),
		Paths:       []string{root},
		ActiveRules: []*api.Rule{rule},
		Format:      "json",
		Version:     "test",
		MaxFixLevel: rules.FixIdiomatic,
	}

	first, err := RunProject(context.Background(), ProjectInput{Args: args, Host: host})
	if err != nil {
		t.Fatalf("first RunProject: %v", err)
	}
	if first.FilesScanned != fileCount {
		t.Fatalf("expected %d files scanned on first call, got %d", fileCount, first.FilesScanned)
	}
	// At 200 files parse always crosses the millisecond floor, so the
	// observability is non-vacuous at this scale.
	if first.PhaseTimingsMs.Parse == 0 {
		t.Errorf("expected parse timing > 0 at %d-file scale; got %+v",
			fileCount, first.PhaseTimingsMs)
	}

	second, err := RunProject(context.Background(), ProjectInput{Args: args, Host: host})
	if err != nil {
		t.Fatalf("second RunProject: %v", err)
	}
	if !second.FindingsBundleHit {
		t.Fatalf("identical %d-file second call must hit the bundle; got hit=false\nphase timings: %+v",
			fileCount, second.PhaseTimingsMs)
	}
	// Bundle hit must skip dispatch+crossfile entirely.
	if second.PhaseTimingsMs.Dispatch != 0 || second.PhaseTimingsMs.CrossFile != 0 {
		t.Errorf("bundle hit must bypass dispatch+crossfile; got dispatch=%dms crossfile=%dms",
			second.PhaseTimingsMs.Dispatch, second.PhaseTimingsMs.CrossFile)
	}
	if second.ParseHits != 0 || second.ParseMisses != 0 {
		t.Errorf("pre-parse bundle hit must not consult parse cache; got hits=%d misses=%d",
			second.ParseHits, second.ParseMisses)
	}
}

func TestRunProject_BundleHitDoesNotAwaitAnalysisCacheFuture(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Foo.kt"),
		[]byte("package demo\n\nclass Foo\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	rule := findV2RuleForTest(t, "UnnecessaryInheritance")
	args := ProjectArgs{
		Config:      config.NewConfig(),
		Paths:       []string{root},
		ActiveRules: []*api.Rule{rule},
		Format:      "json",
		Version:     "test",
	}
	host := ProjectHostState{
		FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
		FindingsBundleCacheRoot: root,
	}
	first, err := RunProject(context.Background(), ProjectInput{Args: args, Host: host})
	if err != nil {
		t.Fatalf("first RunProject: %v", err)
	}
	if first.FindingsBundleHit {
		t.Fatal("first call must miss the bundle")
	}

	calls := 0
	future := NewAnalysisCacheLoadFuture(func() *cache.Cache {
		calls++
		return &cache.Cache{}
	})
	host.AnalysisCacheLoadFuture = future
	second, err := RunProject(context.Background(), ProjectInput{Args: args, Host: host})
	if err != nil {
		t.Fatalf("second RunProject: %v", err)
	}
	if !second.FindingsBundleHit {
		t.Fatal("second call must hit the bundle")
	}
	if calls != 0 {
		t.Fatalf("analysis cache future was awaited on bundle hit; calls=%d", calls)
	}
}
