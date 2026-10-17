package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syntheticHits writes n Kotlin files of roughly 150 lines each, spread over
// a few packages, and returns them as cache hits.
func syntheticHits(b *testing.B, n int) []*CacheEntry {
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
	return hits
}

// BenchmarkDemoteNameCollidingHits measures the per-run cost of the check on
// a warm run with one changed file whose declared names (compute3, sumOf) are
// mentioned by every hit.
func BenchmarkDemoteNameCollidingHits(b *testing.B) {
	hits := syntheticHits(b, 3000)
	miss := filepath.Join(b.TempDir(), "New.kt")
	if err := os.WriteFile(miss, []byte("package app.p3\n\nfun compute3(x: Int): Int = x\nfun neverMentioned(): Int = 0\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demoteNameCollidingHits(InvocationOptions{Backend: BackendKAA}, hits, []string{miss}, nil, false)
	}
}

// BenchmarkDemoteNameCollidingHitsOperatorGates is the worst case: the
// changed file declares several operator functions, whose token gates make
// every hit take the lexical scan, and one is invoke, whose raw gate `(`
// matches every hit. Only hits in the declaring package are demoted.
func BenchmarkDemoteNameCollidingHitsOperatorGates(b *testing.B) {
	hits := syntheticHits(b, 3000)
	miss := filepath.Join(b.TempDir(), "New.kt")
	src := "package app.p3\n\nclass Foo\noperator fun Foo.invoke(): Int = 1\noperator fun Foo.component1(): Int = 1\noperator fun Foo.contains(x: Int): Boolean = true\noperator fun Foo.getValue(t: Any?, p: Any?): Int = 1\n"
	if err := os.WriteFile(miss, []byte(src), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		demoteNameCollidingHits(InvocationOptions{Backend: BackendKAA}, hits, []string{miss}, nil, false)
	}
}
