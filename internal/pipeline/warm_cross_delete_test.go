package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// A deleted file leaves every remaining file a per-file cache hit, but the
// cross-file index changed, so warm cross findings must be recomputed rather
// than replayed from the previous file set (issue #825).
func TestRunProject_WarmCrossFindingsInvalidatedByDeletedFile(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "krit.cache")
	crossCacheDir := scanner.CrossFileCacheDir(dir)
	crossFindingsDir := scanner.CrossFindingsCacheDir(dir)
	bPath := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(filepath.Join(dir, "A.kt"), []byte("package test\n\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("package test\n\nclass B\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Emits one finding per indexed file, so the result depends on the file
	// set and not only on each file's own content.
	var crossCalls int64
	rule := api.FakeRule("CrossRule", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(ctx *api.Context) {
		atomic.AddInt64(&crossCalls, 1)
		for _, f := range ctx.CodeIndex.Files {
			ctx.Emit(scanner.Finding{File: f.Path, Line: 1, Col: 1, Message: "cross"})
		}
	}))
	run := func(t *testing.T) ProjectResult {
		t.Helper()
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{rule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				AnalysisCache:         cache.Load(cachePath),
				AnalysisCacheFilePath: cachePath,
				AnalysisCacheLookup:   true,
				CrossFileCacheDir:     crossCacheDir,
				CrossFindingsCacheDir: crossFindingsDir,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return res
	}

	if first := run(t); first.FindingsCount != 2 {
		t.Fatalf("cold run findings=%d, want 2", first.FindingsCount)
	}
	if second := run(t); second.FindingsCount != 2 {
		t.Fatalf("warm run findings=%d, want 2", second.FindingsCount)
	}
	callsBefore := atomic.LoadInt64(&crossCalls)
	if callsBefore != 1 {
		t.Fatalf("warm run should reuse cross findings; calls=%d want 1", callsBefore)
	}

	if err := os.Remove(bPath); err != nil {
		t.Fatal(err)
	}
	after := run(t)
	if after.FindingsCount != 1 {
		t.Fatalf("after delete findings=%d, want 1 (finding for deleted file must not survive)", after.FindingsCount)
	}
	if got := atomic.LoadInt64(&crossCalls); got != callsBefore+1 {
		t.Fatalf("delete must recompute cross findings; calls=%d want %d", got, callsBefore+1)
	}
}
