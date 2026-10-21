package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syntheticHits writes n Kotlin files of roughly 150 lines each, spread over
// a few packages, and returns them as cache hits along with their directory.
func syntheticHits(b *testing.B, n int) (string, []*CacheEntry) {
	b.Helper()
	dir := b.TempDir()
	hits := make([]*CacheEntry, 0, n)
	for i := 0; i < n; i++ {
		var sb strings.Builder
		fmt.Fprintf(&sb, "package app.p%d\n\nimport kotlin.collections.List\nimport app.p%d.*\n\n", i%7, (i+1)%7)
		for j := 0; j < 30; j++ {
			fmt.Fprintf(&sb, "// helper %d of file %d\nfun f%d_%d(items: List<Int>): Int {\n    val total = items.sumOf { it * %d }\n    return total + compute%d(total)\n}\n\n", j, i, i, j, j, j)
		}
		path := filepath.Join(dir, fmt.Sprintf("File%d.kt", i))
		if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
			b.Fatal(err)
		}
		hits = append(hits, &CacheEntry{FilePath: path})
	}
	return dir, hits
}

// stampedLedger refreshes a ledger over hits and stamps each with its digest.
func stampedLedger(b *testing.B, dir string, hits []*CacheEntry, extra ...string) *declLedger {
	b.Helper()
	paths := make([]string, 0, len(hits)+len(extra))
	for _, h := range hits {
		paths = append(paths, h.FilePath)
	}
	paths = append(paths, extra...)
	l := refreshedLedger(dir, paths)
	for _, h := range hits {
		d, ok := l.currentDigest(h.FilePath)
		if !ok {
			b.Fatalf("cannot digest %s", h.FilePath)
		}
		h.NameDigest = d
	}
	return l
}

// BenchmarkDemoteStaleNameDepsRecompute measures the verification pass on a
// run where a declaration changed, so every hit's digest is recomputed: the
// new file declares compute3, which every hit mentions.
func BenchmarkDemoteStaleNameDepsRecompute(b *testing.B) {
	dir, hits := syntheticHits(b, 3000)
	stampedLedger(b, dir, hits)
	declaring := filepath.Join(dir, "New.kt")
	if err := os.WriteFile(declaring, []byte("package app.p3\n\nfun compute3(x: Int): Int = x\nfun neverMentioned(): Int = 0\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	next := stampedLedger(b, dir, nil, append(hitPaths(hits), declaring)...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, rec := range next.files {
			rec.MemoVersion, rec.MemoDigest = "", ""
		}
		demoteStaleNameDeps(InvocationOptions{Backend: BackendKAA}, next, hits, nil, nil, false)
	}
}

// BenchmarkDemoteStaleNameDepsMemoized is the common warm run: no declaration
// changed, so every digest is served from the ledger's memo.
func BenchmarkDemoteStaleNameDepsMemoized(b *testing.B) {
	dir, hits := syntheticHits(b, 3000)
	l := stampedLedger(b, dir, hits)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demoteStaleNameDeps(InvocationOptions{Backend: BackendKAA}, l, hits, nil, nil, false)
	}
}

// BenchmarkDemoteStaleNameDepsOperatorGates is the worst case for the lexical
// side: the ledger holds several operator functions, whose gates make every
// hit take the scan, and one is invoke, whose raw gate `(` matches every hit.
func BenchmarkDemoteStaleNameDepsOperatorGates(b *testing.B) {
	dir, hits := syntheticHits(b, 3000)
	declaring := filepath.Join(dir, "New.kt")
	src := "package app.p3\n\nclass Foo\noperator fun Foo.invoke(): Int = 1\noperator fun Foo.component1(): Int = 1\noperator fun Foo.contains(x: Int): Boolean = true\noperator fun Foo.getValue(t: Any?, p: Any?): Int = 1\n"
	if err := os.WriteFile(declaring, []byte(src), 0o644); err != nil {
		b.Fatal(err)
	}
	l := stampedLedger(b, dir, nil, append(hitPaths(hits), declaring)...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, rec := range l.files {
			rec.MemoVersion, rec.MemoDigest = "", ""
		}
		demoteStaleNameDeps(InvocationOptions{Backend: BackendKAA}, l, hits, nil, nil, false)
	}
}
