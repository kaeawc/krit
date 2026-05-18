package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cache"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// writeKotlin writes code to a temp file and parses it with scanner.ParseFile
// so FlatTree / NodeTypeTable are populated.
func writeKotlin(t *testing.T, dir, name, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}
	return file
}

func writeJava(t *testing.T, dir, name, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	file, err := scanner.ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ParseJavaFile(%s): %v", path, err)
	}
	return file
}

const classDeclKotlin = "package test\n\nclass X\n"

func TestDispatchPhase_Name(t *testing.T) {
	if got := (DispatchPhase{}).Name(); got != "dispatch" {
		t.Errorf("Name() = %q, want %q", got, "dispatch")
	}
}

func TestDispatchPhase_Run_InvokesRuleOnFile(t *testing.T) {
	file := writeKotlin(t, t.TempDir(), "Sample.kt", classDeclKotlin)

	// Locally-constructed api.Rule — no global Register, so this doesn't
	// pollute the registry (and can be created per-test without duplicate
	// ID panics).
	rule := api.FakeRule("DispatchPhaseTestClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*api.Rule{rule},
			KotlinFiles: []*scanner.File{file},
		},
	}

	out, err := (DispatchPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := out.Findings.Len(); got != 1 {
		t.Fatalf("Findings.Len() = %d, want 1", got)
	}
	if got := out.Findings.RuleAt(0); got != "DispatchPhaseTestClassDecl" {
		t.Errorf("RuleAt(0) = %q, want DispatchPhaseTestClassDecl", got)
	}
}

func TestDispatchPhase_Run_DispatchesJavaFilesToJavaRules(t *testing.T) {
	file := writeJava(t, t.TempDir(), "Sample.java", "package test; class X {}\n")

	rule := api.FakeRule("DispatchPhaseTestJavaClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "java class declared")
		}),
	)
	rule.Languages = []scanner.Language{scanner.LangJava}

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*api.Rule{rule},
			JavaFiles:   []*scanner.File{file},
		},
	}

	out, err := (DispatchPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := out.Findings.Len(); got != 1 {
		t.Fatalf("Findings.Len() = %d, want 1", got)
	}
	if got := out.Findings.RuleAt(0); got != "DispatchPhaseTestJavaClassDecl" {
		t.Errorf("RuleAt(0) = %q, want DispatchPhaseTestJavaClassDecl", got)
	}
}

func TestDispatchPhase_Run_CacheWriteBackIncludesJavaFiles(t *testing.T) {
	dir := t.TempDir()
	file := writeJava(t, dir, "Sample.java", "package test; class X {}\n")

	rule := api.FakeRule("DispatchPhaseTestJavaCacheWrite",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "java class declared")
		}),
	)
	rule.Languages = []scanner.Language{scanner.LangJava}

	analysisCache := &cache.Cache{Files: make(map[string]cache.FileEntry)}
	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*api.Rule{rule},
			JavaFiles:   []*scanner.File{file},
		},
		Cache:         analysisCache,
		CacheFilePath: filepath.Join(dir, ".krit", "cache", cache.CacheFileName),
		Version:       "test",
		RuleHash:      "hash",
	}

	out, err := (DispatchPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := out.Findings.Len(); got != 1 {
		t.Fatalf("Findings.Len() = %d, want 1", got)
	}
	abs, _ := filepath.Abs(file.Path)
	entry, ok := analysisCache.Files[abs]
	if !ok {
		t.Fatalf("cache missing Java entry for %s", abs)
	}
	if got := entry.Columns.Len(); got != 1 {
		t.Fatalf("cached Java findings = %d, want 1", got)
	}
}

func TestDispatchPhase_Run_NoRules_NoFindings(t *testing.T) {
	file := writeKotlin(t, t.TempDir(), "Sample.kt", classDeclKotlin)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: nil,
			KotlinFiles: []*scanner.File{file},
		},
	}

	out, err := (DispatchPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := out.Findings.Len(); got != 0 {
		t.Errorf("Findings.Len() = %d, want 0", got)
	}
}

func TestDispatchPhase_Run_SkipsCachedFiles(t *testing.T) {
	dir := t.TempDir()
	cached := writeKotlin(t, dir, "Cached.kt", classDeclKotlin)
	fresh := writeKotlin(t, dir, "Fresh.kt", classDeclKotlin)

	rule := api.FakeRule("DispatchPhaseTestCacheSkip",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(1, 1, "found")
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*api.Rule{rule},
			KotlinFiles: []*scanner.File{cached, fresh},
		},
		CacheResult: &cache.Result{
			CachedPaths: map[string]bool{cached.Path: true},
		},
	}

	out, err := (DispatchPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := out.Findings.Len(); got != 1 {
		t.Fatalf("Findings.Len() = %d, want 1 (only fresh file)", got)
	}
	if got := out.Findings.FileAt(0); got != fresh.Path {
		t.Errorf("FileAt(0) = %q, want %q", got, fresh.Path)
	}
}

func TestCanSkipCacheSave_FullyWarmParseSkipped(t *testing.T) {
	in := IndexResult{
		Cache: &cache.Cache{Files: make(map[string]cache.FileEntry)},
		CacheResult: &cache.Result{
			CachedPaths: map[string]bool{"A.kt": true},
			TotalCached: 1,
			TotalFiles:  1,
		},
	}
	if !canSkipCacheSave(in) {
		t.Fatal("expected cache save skip when every file is cached and no sources were parsed")
	}
}

func TestCanSkipCacheSave_BlockedByParsedMiss(t *testing.T) {
	file := writeKotlin(t, t.TempDir(), "Fresh.kt", classDeclKotlin)
	in := IndexResult{
		ParseResult: ParseResult{KotlinFiles: []*scanner.File{file}},
		Cache:       &cache.Cache{Files: make(map[string]cache.FileEntry)},
		CacheResult: &cache.Result{
			CachedPaths: map[string]bool{},
			TotalCached: 0,
			TotalFiles:  1,
		},
	}
	if canSkipCacheSave(in) {
		t.Fatal("cache save skip should be blocked when a parsed source may update the cache")
	}
}

func TestDispatchPhase_Run_ContextCancel(t *testing.T) {
	file := writeKotlin(t, t.TempDir(), "Sample.kt", classDeclKotlin)

	rule := api.FakeRule("DispatchPhaseTestCtxCancel",
		api.WithNodeTypes("class_declaration"),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(1, 1, "should not run")
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*api.Rule{rule},
			KotlinFiles: []*scanner.File{file},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (DispatchPhase{}).Run(ctx, in)
	if err == nil {
		t.Fatal("Run: expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run: err = %v, want context.Canceled", err)
	}
}

func TestDispatchPhase_Run_ParallelDoesNotDeadlock(t *testing.T) {
	const n = 50
	dir := t.TempDir()
	files := make([]*scanner.File, 0, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("File%02d.kt", i)
		files = append(files, writeKotlin(t, dir, name, classDeclKotlin))
	}

	var checked atomic.Int64
	rule := api.FakeRule("DispatchPhaseTestParallel",
		api.WithNodeTypes("class_declaration"),
		api.WithCheck(func(ctx *api.Context) {
			checked.Add(1)
			ctx.EmitAt(1, 1, "class")
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*api.Rule{rule},
			KotlinFiles: files,
		},
	}

	out, err := (DispatchPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := out.Findings.Len(); got != n {
		t.Errorf("Findings.Len() = %d, want %d", got, n)
	}
	if got := checked.Load(); got != n {
		t.Errorf("check invocations = %d, want %d", got, n)
	}
}

// TestDispatchPhase_Run_StopsQueuedFilesOnContextCancel verifies that
// once the caller cancels the dispatch context, queued (not-yet-
// started) file-level goroutines bail out without running the
// dispatcher. errgroup.SetLimit serialises goroutine starts; without
// the cancellation guard, every queued worker would still invoke
// RunWithStats even when the caller already gave up.
func TestDispatchPhase_Run_StopsQueuedFilesOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	const n = 64
	files := make([]*scanner.File, 0, n)
	for i := 0; i < n; i++ {
		files = append(files, writeKotlin(t, dir, fmt.Sprintf("F%d.kt", i), classDeclKotlin))
	}

	var (
		ran     atomic.Int64
		release = make(chan struct{})
	)
	rule := api.FakeRule("DispatchCancelObserver",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ran.Add(1)
			// Block the first goroutines so the cancellation has time
			// to fire before the queue drains. release unblocks them
			// once the test cancels the context.
			<-release
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*api.Rule{rule},
			KotlinFiles: files,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	type result struct {
		out DispatchResult
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		out, err := (DispatchPhase{}).Run(ctx, in)
		resultCh <- result{out: out, err: err}
	}()

	// Wait for some workers to start, then cancel. The remaining queued
	// files should never enter the rule callback after the guard sees
	// the cancellation.
	deadline := time.After(2 * time.Second)
	for ran.Load() == 0 {
		select {
		case <-deadline:
			cancel()
			close(release)
			<-resultCh
			t.Fatalf("no workers started before deadline")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	close(release)
	res := <-resultCh

	if !errors.Is(res.err, context.Canceled) {
		t.Fatalf("Run err = %v, want context.Canceled", res.err)
	}
	if got := ran.Load(); got >= int64(n) {
		t.Fatalf("rule callback ran on %d/%d files; expected fewer once context was cancelled", got, n)
	}
}
