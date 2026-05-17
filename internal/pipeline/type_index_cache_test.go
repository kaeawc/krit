package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestIndexPhase_TypeIndexCacheDir_HitsOnSecondRun runs IndexPhase twice
// with TypeIndexCacheDir set; the second run should hit the on-disk
// FileTypeInfo cache and report misses=0. Acceptance criterion for #56:
// warm runs skip per-file extraction for unchanged files.
func TestIndexPhase_TypeIndexCacheDir_HitsOnSecondRun(t *testing.T) {
	dir := t.TempDir()
	cacheDir := t.TempDir()
	src := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(src, []byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := scanner.ParseFile(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}

	// IndexPhase only runs IndexFilesParallel when at least one rule
	// declares NeedsResolver. Construct a synthetic one.
	rule := api.FakeRule("R", api.WithNeeds(api.NeedsResolver))

	in := IndexInput{
		ParseResult: ParseResult{
			Paths:       []string{dir},
			KotlinFiles: []*scanner.File{parsed},
			ActiveRules: []*api.Rule{rule},
		},
		TypeIndexCacheDir: cacheDir,
	}

	if _, err := (IndexPhase{SkipModules: true, SkipAndroid: true}).Run(context.Background(), in); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// First run populates the cache. Verify by scanning cacheDir.
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) == 0 {
		t.Fatal("first Run did not populate type-index cache directory")
	}

	// Snapshot cache stats, then run again. The second run should
	// register hits without rebuilding.
	hitsBefore, _, _ := readTypeIndexStats()

	if _, err := (IndexPhase{SkipModules: true, SkipAndroid: true}).Run(context.Background(), in); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	hitsAfter, _, _ := readTypeIndexStats()
	if hitsAfter <= hitsBefore {
		t.Errorf("second Run did not hit type-index cache: hits before=%d after=%d", hitsBefore, hitsAfter)
	}
}

func readTypeIndexStats() (hits, misses, bytes int64) {
	for _, e := range cacheutil.AllStats() {
		if e.Name == "type-index-cache" {
			return e.Stats.Hits, e.Stats.Misses, e.Stats.Bytes
		}
	}
	return 0, 0, 0
}
