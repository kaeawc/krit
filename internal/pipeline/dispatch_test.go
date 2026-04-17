package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
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

	// Locally-constructed v2.Rule — no global Register, so this doesn't
	// pollute the registry (and can be created per-test without duplicate
	// ID panics).
	rule := v2.FakeRule("DispatchPhaseTestClassDecl",
		v2.WithNodeTypes("class_declaration"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*v2.Rule{rule},
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

	rule := v2.FakeRule("DispatchPhaseTestCacheSkip",
		v2.WithNodeTypes("class_declaration"),
		v2.WithSeverity(v2.SeverityWarning),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(1, 1, "found")
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*v2.Rule{rule},
			KotlinFiles: []*scanner.File{cached, fresh},
		},
		CacheResult: &cache.CacheResult{
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

func TestDispatchPhase_Run_ContextCancel(t *testing.T) {
	file := writeKotlin(t, t.TempDir(), "Sample.kt", classDeclKotlin)

	rule := v2.FakeRule("DispatchPhaseTestCtxCancel",
		v2.WithNodeTypes("class_declaration"),
		v2.WithCheck(func(ctx *v2.Context) {
			ctx.EmitAt(1, 1, "should not run")
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*v2.Rule{rule},
			KotlinFiles: []*scanner.File{file},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (DispatchPhase{}).Run(ctx, in)
	if err == nil {
		t.Fatal("Run: expected error from cancelled context, got nil")
	}
	if err != context.Canceled {
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
	rule := v2.FakeRule("DispatchPhaseTestParallel",
		v2.WithNodeTypes("class_declaration"),
		v2.WithCheck(func(ctx *v2.Context) {
			checked.Add(1)
			ctx.EmitAt(1, 1, "class")
		}),
	)

	in := IndexResult{
		ParseResult: ParseResult{
			ActiveRules: []*v2.Rule{rule},
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
