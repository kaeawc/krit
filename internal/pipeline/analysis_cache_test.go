package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRunProject_AnalysisCacheWriteBack confirms that when Host carries
// an AnalysisCache + AnalysisCacheFilePath, RunProject populates the
// cache with per-file findings and persists it to disk. Lookup-side
// (file-skip on hit) is intentionally out of scope for this PR; the
// write-only contract verified here is enough for subsequent CLI runs
// to benefit from the daemon-populated cache.
func TestRunProject_AnalysisCacheWriteBack(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".krit", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "krit.cache")
	src := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(src, []byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("ProjectSmokeClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	loaded := cache.Load(cachePath)
	if loaded == nil {
		t.Fatal("cache.Load returned nil")
	}

	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
		Host: ProjectHostState{
			AnalysisCache:         loaded,
			AnalysisCacheFilePath: cachePath,
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.FindingsCount != 1 {
		t.Errorf("FindingsCount = %d, want 1", res.FindingsCount)
	}

	// Reload from disk and confirm the cache file persisted with a
	// non-empty entry for the dispatched file.
	reloaded := cache.Load(cachePath)
	if reloaded == nil {
		t.Fatal("reload: cache.Load returned nil")
	}
	if len(reloaded.Files) == 0 {
		t.Fatalf("expected cache to be populated after dispatch; got %d entries", len(reloaded.Files))
	}
	if reloaded.RuleHash == "" {
		t.Errorf("expected RuleHash to be set after dispatch")
	}
	if reloaded.Version != "test" {
		t.Errorf("Version = %q, want \"test\"", reloaded.Version)
	}
}

func TestRunProject_AnalysisCacheLoadFutureWriteBack(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "krit.cache")
	src := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(src, []byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("ProjectSmokeClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	calls := 0
	future := NewAnalysisCacheLoadFuture(func() *cache.Cache {
		calls++
		return cache.Load(cachePath)
	})
	future.Start()

	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
		Host: ProjectHostState{
			AnalysisCacheLoadFuture: future,
			AnalysisCacheFilePath:   cachePath,
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.FindingsCount != 1 {
		t.Errorf("FindingsCount = %d, want 1", res.FindingsCount)
	}
	if calls != 1 {
		t.Errorf("future load calls = %d, want 1", calls)
	}
	if reloaded := cache.Load(cachePath); reloaded == nil || len(reloaded.Files) == 0 {
		t.Fatalf("expected future-loaded cache to be persisted, got %#v", reloaded)
	}
}

func TestRunProject_AnalysisCacheLoadFutureFallbackOnPanic(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "krit.cache")
	src := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(src, []byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("ProjectSmokeClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
		Host: ProjectHostState{
			AnalysisCacheLoadFuture: NewAnalysisCacheLoadFuture(func() *cache.Cache {
				panic("boom")
			}),
			AnalysisCacheFilePath: cachePath,
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.FindingsCount != 1 {
		t.Errorf("FindingsCount = %d, want 1", res.FindingsCount)
	}
	if reloaded := cache.Load(cachePath); reloaded == nil || len(reloaded.Files) == 0 {
		t.Fatalf("expected fallback-loaded cache to be persisted, got %#v", reloaded)
	}
}
