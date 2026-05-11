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

// TestRunProject_AnalysisCacheLookup_ByteEqualAcrossRuns is the
// byte-equal contract test #126 calls out: a second RunProject call
// with lookup enabled must produce output indistinguishable from the
// first (cold) call. This is the safety net for the daemon-resident
// AnalysisCache lookup path — if cache hits + cache misses don't
// reassemble identically, the daemon's output silently drifts from
// the CLI's.
func TestRunProject_AnalysisCacheLookup_ByteEqualAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".krit", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "krit.cache")

	// Write a few synthetic files so cache lookup has something to
	// actually classify.
	for _, name := range []string{"A.kt", "B.kt", "C.kt"} {
		if err := os.WriteFile(filepath.Join(dir, name),
			[]byte("package test\n\nclass "+name[:1]+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	run := func(t *testing.T) []byte {
		t.Helper()
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
				AnalysisCacheLookup:   true,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return stripFindingTimestamps(res.JSON)
	}

	first := run(t)
	second := run(t)
	if string(first) != string(second) {
		t.Errorf("byte-equal contract failed:\n--- first ---\n%s\n--- second ---\n%s",
			first, second)
	}
}

// TestRunProject_AnalysisCacheLookup_DisabledByDefault verifies that
// without AnalysisCacheLookup=true, RunProject keeps the pre-#126
// write-only behavior — every file is dispatched on every call,
// regardless of cache contents.
func TestRunProject_AnalysisCacheLookup_DisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "krit.cache")
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"),
		[]byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	loaded := cache.Load(cachePath)
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
			// AnalysisCacheLookup intentionally false.
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.FindingsCount != 1 {
		t.Errorf("FindingsCount = %d, want 1 (rule must dispatch when lookup disabled)", res.FindingsCount)
	}
}

func TestRunProject_AnalysisCacheLookup_FullyWarmSkipsParseAndDispatch(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "krit.cache")
	for _, name := range []string{"A.kt", "B.kt"} {
		if err := os.WriteFile(filepath.Join(dir, name),
			[]byte("package test\n\nclass "+name[:1]+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var calls int64
	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			atomic.AddInt64(&calls, 1)
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

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
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return res
	}

	first := run(t)
	if first.FilesScanned != 2 || atomic.LoadInt64(&calls) != 2 {
		t.Fatalf("cold run scanned=%d calls=%d, want scanned=2 calls=2", first.FilesScanned, calls)
	}
	second := run(t)
	if second.FilesScanned != 2 {
		t.Fatalf("warm run FilesScanned=%d, want 2", second.FilesScanned)
	}
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Fatalf("warm run dispatched cached files; calls=%d want 2", got)
	}
}

func TestRunProject_AnalysisCacheLookup_ParsesOnlyMisses(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "krit.cache")
	aPath := filepath.Join(dir, "A.kt")
	bPath := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(aPath, []byte("package test\n\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("package test\n\nclass B\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls int64
	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			atomic.AddInt64(&calls, 1)
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)
	run := func(t *testing.T) {
		t.Helper()
		_, err := RunProject(context.Background(), ProjectInput{
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
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
	}

	run(t)
	if err := os.WriteFile(bPath, []byte("package test\n\nclass B2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t)
	if got := atomic.LoadInt64(&calls); got != 3 {
		t.Fatalf("second run should dispatch only the edited file; calls=%d want 3", got)
	}
}

func TestRunProject_WarmCrossFindingsSkipCrossRules(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "krit.cache")
	crossCacheDir := scanner.CrossFileCacheDir(dir)
	crossFindingsDir := scanner.CrossFindingsCacheDir(dir)
	if err := os.WriteFile(filepath.Join(dir, "A.kt"), []byte("package test\n\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var crossCalls int64
	rule := api.FakeRule("CrossRule", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(ctx *api.Context) {
		atomic.AddInt64(&crossCalls, 1)
		ctx.Emit(scanner.Finding{File: filepath.Join(dir, "A.kt"), Line: 1, Col: 1, Message: "cross"})
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

	first := run(t)
	if first.FindingsCount != 1 || atomic.LoadInt64(&crossCalls) != 1 {
		t.Fatalf("cold run findings=%d crossCalls=%d, want 1/1", first.FindingsCount, crossCalls)
	}
	second := run(t)
	if second.FindingsCount != 1 {
		t.Fatalf("warm run findings=%d, want 1", second.FindingsCount)
	}
	if got := atomic.LoadInt64(&crossCalls); got != 1 {
		t.Fatalf("warm run re-ran cross rule; calls=%d want 1", got)
	}
}

// stripFindingTimestamps drops timing-related JSON fields that change
// across runs so byte-equal comparisons exercise just the findings,
// not the wall-clock fluctuations.
func stripFindingTimestamps(jsonBytes []byte) []byte {
	// Wall durations and timestamps differ across runs even with
	// identical findings. JSON output embeds `"durationMs":...` and
	// `"timestamp":"..."` near the top. We strip naively by line —
	// JSON output is pretty-printed in this codebase, so per-field
	// stripping is robust enough for the byte-equal contract.
	return scrubLines(jsonBytes, []string{
		"\"durationMs\"",
		"\"timestamp\"",
		"\"wallSeconds\"",
		"\"startTime\"",
		"\"endTime\"",
	})
}

func scrubLines(data []byte, dropTokens []string) []byte {
	const newline = byte('\n')
	out := make([]byte, 0, len(data))
	start := 0
	for i := 0; i <= len(data); i++ {
		if i != len(data) && data[i] != newline {
			continue
		}
		line := data[start:i]
		drop := false
		for _, tok := range dropTokens {
			if bytesContains(line, []byte(tok)) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, line...)
			if i != len(data) {
				out = append(out, newline)
			}
		}
		start = i + 1
	}
	return out
}

func bytesContains(hay, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		match := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
