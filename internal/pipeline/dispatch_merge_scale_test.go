package pipeline

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// dispatchMergeBytes runs the dispatch phase over n files that each report a few findings with distinct messages,
// and returns the bytes allocated by the run.
func dispatchMergeBytes(t *testing.T, n int) uint64 {
	t.Helper()
	dir := t.TempDir()
	files := make([]*scanner.File, 0, n)
	for i := range n {
		files = append(files, writeKotlin(t, dir, fmt.Sprintf("File%04d.kt", i), "package test\n\nclass A\nclass B\nclass C\n"))
	}
	rule := api.FakeRule("DispatchPhaseTestMergeScale",
		api.WithNodeTypes("class_declaration"),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class in "+ctx.File.Path)
		}),
	)
	in := IndexResult{ParseResult: ParseResult{ActiveRules: []*api.Rule{rule}, KotlinFiles: files}}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	out, err := (DispatchPhase{}).Run(context.Background(), in)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := out.Findings.Len(), 3*n; got != want {
		t.Fatalf("Findings.Len() = %d, want %d", got, want)
	}
	return after.TotalAlloc - before.TotalAlloc
}

// TestDispatchPhase_Run_FindingsMergeScalesLinearly guards the per-file merge of findings into the run's columns:
// it once rebuilt the merged columns from scratch for every file, so the work grew with files x findings. With 4x
// the files a linear merge allocates about 4x the bytes; the quadratic one allocated over 10x.
func TestDispatchPhase_Run_FindingsMergeScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a few thousand files")
	}
	const small, large = 300, 1200
	smallBytes := dispatchMergeBytes(t, small)
	largeBytes := dispatchMergeBytes(t, large)
	ratio := float64(largeBytes) / float64(smallBytes)
	t.Logf("allocated %d bytes for %d files, %d bytes for %d files (ratio %.1f)", smallBytes, small, largeBytes, large, ratio)
	if ratio > 7 {
		t.Fatalf("allocation grew %.1fx for %dx the files; the findings merge is doing per-file work over every finding",
			ratio, large/small)
	}
}
