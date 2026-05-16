package javafacts

import (
	"fmt"
	"testing"

	"github.com/kaeawc/krit/internal/corpustest"
	"github.com/kaeawc/krit/internal/scanner"
)

// BenchmarkSourceIndexKey_LargeCorpus measures the key-computation
// cost SourceIndexForFiles pays on every call — including warm
// cache hits, since the key has to be computed before the lookup.
// The Kotlin compiler corpus has ~2k Java files; this bench builds
// a synthetic equivalent so the measurement doesn't depend on a
// real checkout.
func BenchmarkSourceIndexKey_LargeCorpus(b *testing.B) {
	files := buildSyntheticJavaFiles(2000, 5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sourceIndexKey(files)
	}
}

// BenchmarkSourceIndexForFiles_WarmHit measures the steady-state
// cost: cache populated, every call short-circuits to the existing
// cache entry. Bench shows what users actually pay on every ABI
// edit when neither Kotlin nor Java source changes affect the
// Java source index.
func BenchmarkSourceIndexForFiles_WarmHit(b *testing.B) {
	files := buildSyntheticJavaFiles(2000, 10000)
	// Prime the cache.
	_ = SourceIndexForFiles(files)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SourceIndexForFiles(files)
	}
}

// BenchmarkSourceIndexForFiles_WarmHit_WithKotlin mirrors the
// production call shape where parsedFiles also includes the much
// larger Kotlin source set: the key build walks every entry to
// filter out Java files first.
func BenchmarkSourceIndexForFiles_WarmHit_WithKotlin(b *testing.B) {
	javaFiles := buildSyntheticJavaFiles(2000, 10000)
	kotlinFiles := buildSyntheticKotlinFiles(13500)
	mixed := append([]*scanner.File(nil), kotlinFiles...)
	mixed = append(mixed, javaFiles...)
	_ = SourceIndexForFiles(mixed)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SourceIndexForFiles(mixed)
	}
}

func buildSyntheticKotlinFiles(n int) []*scanner.File {
	out := make([]*scanner.File, n)
	corpus := corpustest.KotlinCorpusPath()
	for i := 0; i < n; i++ {
		out[i] = &scanner.File{
			Path:     fmt.Sprintf("%s/some/kotlin/File%d.kt", corpus, i),
			Language: scanner.LangKotlin,
			Content:  []byte("package demo\nclass C\n"),
		}
	}
	return out
}

func buildSyntheticJavaFiles(n, contentBytes int) []*scanner.File {
	out := make([]*scanner.File, n)
	content := make([]byte, contentBytes)
	for i := range content {
		content[i] = byte('A' + (i % 26))
	}
	corpus := corpustest.KotlinCorpusPath()
	for i := 0; i < n; i++ {
		out[i] = &scanner.File{
			Path:     fmt.Sprintf("%s/some/dir/File%d.java", corpus, i),
			Language: scanner.LangJava,
			Content:  content,
		}
	}
	return out
}
