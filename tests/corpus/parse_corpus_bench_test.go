// Package corpus_test runs benchmarks against external source
// corpora (the Kotlin compiler, Signal-Android, etc.). Tests in
// this package are GATED on the KOTLIN_CORPUS / SIGNAL_ANDROID_CORPUS
// env variables (see internal/corpustest + .env.example) and
// t.Skip when those are unset, so the package is a no-op on
// machines that don't have the checkouts.
//
// Why a separate package: the tests need to import internal/scanner
// to do real parsing, which keeps the dependency graph clean
// (corpustest stays stdlib-only) and isolates "I want to bench
// against my actual repo" workloads from the synthetic benches
// that ship with each package.
package corpus_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/corpustest"
	"github.com/kaeawc/krit/internal/fileignore"
	"github.com/kaeawc/krit/internal/scanner"
)

// BenchmarkParseKotlinCorpus exercises scanner.ParseFile against
// every .kt file under KOTLIN_CORPUS. Acts as both a parse-
// throughput measurement on real-world code AND as the first
// non-test caller of corpustest.RequireKotlinCorpus — i.e. the
// integration test that proves the helper works end-to-end.
//
// Runtime on the JetBrains/kotlin corpus (~18 k files) is tens of
// seconds per iteration, so run with -benchtime=1x for a single
// pass when you just want a number:
//
//	KOTLIN_CORPUS=/path/to/kotlin \
//	    go test -bench=BenchmarkParseKotlinCorpus -benchtime=1x ./tests/corpus/
//
// Or with -count=N to amortize startup costs across repeated runs.
func BenchmarkParseKotlinCorpus(b *testing.B) {
	root := corpustest.RequireKotlinCorpus(b)
	paths := collectKotlinFiles(b, root)
	if len(paths) == 0 {
		b.Skipf("no .kt files found under %s", root)
	}

	b.ResetTimer()

	var bytesRead int64
	var parseErrors int
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			f, err := scanner.ParseFile(p)
			if err != nil {
				// Real corpora occasionally contain test fixtures
				// the parser intentionally rejects. Don't fail
				// the bench on per-file parse errors — track them
				// in a metric instead.
				parseErrors++
				continue
			}
			bytesRead += int64(len(f.Content))
		}
	}
	// Per-iteration metrics. b.N defaults to 1 for this bench at
	// -benchtime=1x; for higher counts the per-op normalization
	// keeps the numbers comparable.
	b.ReportMetric(float64(len(paths)), "files/op")
	b.ReportMetric(float64(bytesRead)/float64(b.N)/1024/1024, "MB/op")
	if parseErrors > 0 {
		b.ReportMetric(float64(parseErrors)/float64(b.N), "parse-errors/op")
	}
}

// collectKotlinFiles walks root for .kt files, pruning the
// directories fileignore.DefaultPrunedDir excludes (.git, build,
// .gradle, node_modules, ...). Pre-collected so the bench timer
// doesn't include the filesystem walk.
func collectKotlinFiles(tb testing.TB, root string) []string {
	tb.Helper()
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip-and-continue: walk errors on per-entry stat shouldn't fail the bench setup
		}
		if info.IsDir() {
			if path != root && fileignore.DefaultPrunedDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".kt") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		tb.Fatalf("walk %s: %v", root, err)
	}
	return paths
}
