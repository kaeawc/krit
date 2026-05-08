package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// HitRateExpectation captures the warm-cache invariants a fixture is
// expected to honor. It is consumed by AssertWarmHitRate and lets test
// authors lock in the cache's behavior on a known input set.
type HitRateExpectation struct {
	// TotalFiles is the number of files passed to CheckFiles.
	TotalFiles int
	// MinCached is the minimum hit count that must be observed. Use it for
	// "at least N files must hit on the warm pass" assertions.
	MinCached int
	// ExpectAllHit, when true, requires every file to hit (TotalCached ==
	// TotalFiles). Equivalent to MinCached = TotalFiles.
	ExpectAllHit bool
}

// AssertWarmHitRate is a reusable harness that builds a cache from a
// fixture, then runs CheckFiles a second time and verifies the warm-pass
// hit-rate matches the expectation. Any future change that accidentally
// reduces reuse will fail here.
//
// fixture maps relative file paths (under the test's TempDir) to file
// contents. The cache is populated with entries for every fixture file
// before the warm CheckFiles call.
func AssertWarmHitRate(t *testing.T, fixture map[string]string, exp HitRateExpectation) {
	t.Helper()
	root := t.TempDir()

	var paths []string
	for rel, body := range fixture {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", abs, err)
		}
		if err := os.WriteFile(abs, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", abs, err)
		}
		paths = append(paths, abs)
	}

	c := &Cache{
		Version:   "1.0.0",
		RuleHash:  "rh",
		ScanPaths: []string{root},
		Files:     make(map[string]FileEntry),
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		c.Files[p] = FileEntry{
			Hash:    ComputeFileHash(p),
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
			Columns: scanner.CollectFindings(nil),
		}
	}

	res := c.CheckFiles(paths, "rh", root)
	if res.TotalFiles != exp.TotalFiles {
		t.Errorf("TotalFiles: got %d, want %d", res.TotalFiles, exp.TotalFiles)
	}
	wantMin := exp.MinCached
	if exp.ExpectAllHit {
		wantMin = exp.TotalFiles
	}
	if res.TotalCached < wantMin {
		t.Errorf("warm hit-rate regression: got %d/%d cached, want >= %d", res.TotalCached, res.TotalFiles, wantMin)
	}
}

// TestWarmHitRate_TrivialFixture demonstrates the harness on a small,
// fully-cached fixture. Use this as a template when wiring the harness
// into other test packages.
func TestWarmHitRate_TrivialFixture(t *testing.T) {
	AssertWarmHitRate(t, map[string]string{
		"a.kt":     "fun a() {}\n",
		"b.kt":     "fun b() {}\n",
		"sub/c.kt": "fun c() {}\n",
	}, HitRateExpectation{
		TotalFiles:   3,
		ExpectAllHit: true,
	})
}

// TestWarmHitRate_PartialMiss demonstrates the harness when some files
// are expected to miss (e.g. mid-edit). Here, mutating one file after
// cache population ensures it's a miss; the other two should still hit.
func TestWarmHitRate_PartialMiss(t *testing.T) {
	root := t.TempDir()
	fixture := map[string]string{
		"a.kt": "fun a() {}\n",
		"b.kt": "fun b() {}\n",
		"c.kt": "fun c() {}\n",
	}
	var paths []string
	for rel, body := range fixture {
		abs := filepath.Join(root, rel)
		if err := os.WriteFile(abs, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, abs)
	}

	c := &Cache{
		Version:  "1.0.0",
		RuleHash: "rh",
		Files:    make(map[string]FileEntry),
	}
	for _, p := range paths {
		info, _ := os.Stat(p)
		c.Files[p] = FileEntry{
			Hash:    ComputeFileHash(p),
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
		}
	}

	// Mutate one file after cache population.
	target := filepath.Join(root, "b.kt")
	if err := os.WriteFile(target, []byte("fun b() { val x = 1 }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	res := c.CheckFiles(paths, "rh", root)
	if res.TotalFiles != 3 {
		t.Errorf("TotalFiles=%d, want 3", res.TotalFiles)
	}
	if res.TotalCached != 2 {
		t.Errorf("TotalCached=%d, want 2 (one file mutated)", res.TotalCached)
	}
	if res.CachedPaths[target] {
		t.Errorf("mutated file %s should not be cached", target)
	}
}
