//go:build kotlin_corpus

// Package serve benchmarks for the analyze-project verb against a
// real Kotlin corpus checkout. Build-tagged so the default test
// run (and CI) skip these — corpus availability is local-only.
//
// To run:
//
//	export KRIT_KOTLIN_CORPUS=/path/to/kotlin
//	go test -tags=kotlin_corpus -run='^$' \
//	    -bench=BenchmarkAnalyzeProject \
//	    -benchtime=10x \
//	    ./internal/cli/serve/
//
// The benchmarks here exist to establish the speed claim from PR
// #36's follow-up: warm-cache analyze-project should be ≤ 2s on
// JetBrains/kotlin (~63k files), versus the 9.5s one-shot CLI
// baseline.
package serve

import (
	"os"
	"runtime"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

func corpusRoot(b *testing.B) string {
	b.Helper()
	root := os.Getenv("KRIT_KOTLIN_CORPUS")
	if root == "" {
		b.Skip("KRIT_KOTLIN_CORPUS env var not set; set to a Kotlin corpus checkout to enable")
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		b.Skipf("KRIT_KOTLIN_CORPUS=%q is not a directory: %v", root, err)
	}
	return root
}

// BenchmarkAnalyzeProjectCold reports the first-call cost. This is
// the warm-up cost the daemon pays once per lifetime; subsequent
// runs amortize.
func BenchmarkAnalyzeProjectCold(b *testing.B) {
	root := corpusRoot(b)

	for i := 0; i < b.N; i++ {
		// Fresh daemon per iteration so each measurement is a true
		// cold-start cost (no in-memory parse cache reuse from a
		// prior run).
		socket, _ := startServerForCorpus(b, root)
		var got daemon.AnalyzeProjectResult
		b.StartTimer()
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{}, &got); err != nil {
			b.Fatalf("call: %v", err)
		}
		b.StopTimer()
		b.ReportMetric(got.Stats.WallSeconds, "wall_seconds")
		b.ReportMetric(float64(got.Stats.FilesScanned), "files_scanned")
		b.ReportMetric(float64(got.Stats.FindingsCount), "findings_count")
	}
}

// BenchmarkAnalyzeProjectWarm reports the steady-state cost: a
// single daemon instance, b.N invocations against unchanged input.
// The first iteration warms; the reported time per op should
// converge after the first sample.
func BenchmarkAnalyzeProjectWarm(b *testing.B) {
	root := corpusRoot(b)
	socket, _ := startServerForCorpus(b, root)

	// Warm-up: one untimed call.
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &daemon.AnalyzeProjectResult{}); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var got daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{}, &got); err != nil {
			b.Fatalf("iter %d: %v", i, err)
		}
		if i == b.N-1 {
			b.ReportMetric(got.Stats.WallSeconds, "wall_seconds")
		}
	}
}

// BenchmarkAnalyzeProjectLeak measures heap growth across many
// warm calls. A regression that leaks state per call would show up
// as monotonically increasing HeapInuse.
//
// Reports the heap delta between the first and last warm call as
// a custom metric "heap_growth_bytes". A passing benchmark prints
// a small number (or negative when the GC freed slack from the
// warm-up); a failing one shows hundreds of MB of growth.
func BenchmarkAnalyzeProjectLeak(b *testing.B) {
	root := corpusRoot(b)
	socket, _ := startServerForCorpus(b, root)

	// Warm: bring the parse cache into RAM and stabilize the JIT.
	for i := 0; i < 3; i++ {
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{}, &daemon.AnalyzeProjectResult{}); err != nil {
			b.Fatalf("warm %d: %v", i, err)
		}
	}

	var ms0, ms1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{}, &daemon.AnalyzeProjectResult{}); err != nil {
			b.Fatalf("iter %d: %v", i, err)
		}
	}
	b.StopTimer()

	runtime.GC()
	runtime.ReadMemStats(&ms1)

	growth := int64(ms1.HeapInuse) - int64(ms0.HeapInuse)
	b.ReportMetric(float64(growth), "heap_growth_bytes")
	b.ReportMetric(float64(ms1.HeapInuse), "final_heap_bytes")

	// 200 MB growth budget across N iterations is the loose CI-
	// friendly bar from the daemon plan. A failure here means a
	// real leak; tune up only with a memory profile to back the
	// new ceiling.
	const budget int64 = 200 * 1024 * 1024
	if growth > budget {
		b.Errorf("heap grew %d bytes across %d iters (budget %d)", growth, b.N, budget)
	}
}

// startServerForCorpus is the corpus-rooted wrapper over
// startServerWith. readyTimeout=0 so the bench tight-spins on
// daemon.Available — startup cost isn't part of the measurement.
func startServerForCorpus(b *testing.B, root string) (string, *daemonState) {
	b.Helper()
	return startServerWith(b, root, 0)
}
