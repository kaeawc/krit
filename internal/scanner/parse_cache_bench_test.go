package scanner

// Parametric sweep for parseCacheMinFileSize. For each file-size bucket
// we measure three hot paths:
//
//   parse-only:   tree-sitter parse + flatten, no cache I/O
//   cache-miss:   parse-only + gob-encode + atomic write
//   cache-hit:    gob-decode + node-type remap (no parse)
//
// The crossover where cache-hit < parse-only tells us the smallest file
// size at which caching pays off on subsequent runs. The cache-miss row
// is the amortized first-run penalty for engaging the cache.
//
// Run with:
//
//   go test ./internal/scanner -bench=BenchmarkParseCacheSweep \
//     -run=^$ -benchtime=200x -count=5
//
// See parseCacheMinFileSize in parse_cache.go.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var parseCacheSweepSizes = []int{256, 512, 1024, 2048, 4096}

// buildKotlinSourceAtLeast returns a Kotlin source blob whose byte
// length is >= target. Grown by appending short functions so the AST
// shape stays representative of real code rather than a single blob of
// whitespace.
func buildKotlinSourceAtLeast(target int) string {
	var b strings.Builder
	b.WriteString("package bench\n\n")
	i := 0
	for b.Len() < target {
		fmt.Fprintf(&b, "fun f%d(x: Int): Int = x + %d\n", i, i)
		i++
	}
	return b.String()
}

func BenchmarkParseCacheSweep_ParseOnly(b *testing.B) {
	for _, size := range parseCacheSweepSizes {
		size := size
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			src := []byte(buildKotlinSourceAtLeast(size))
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parser := GetKotlinParser()
				tree, err := parser.ParseCtx(context.Background(), nil, src)
				if err != nil {
					b.Fatal(err)
				}
				_ = flattenTree(tree.RootNode())
				PutKotlinParser(parser)
			}
		})
	}
}

func BenchmarkParseCacheSweep_CacheMiss(b *testing.B) {
	for _, size := range parseCacheSweepSizes {
		size := size
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			repo := b.TempDir()
			pc, err := NewParseCache(repo)
			if err != nil {
				b.Fatalf("NewParseCache: %v", err)
			}
			src := []byte(buildKotlinSourceAtLeast(size))
			path := filepath.Join(repo, "bench.kt")
			if err := os.WriteFile(path, src, 0o644); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Force every iteration to be a miss by clearing the
				// entry dir. We measure parse + encode + write-atomic.
				b.StopTimer()
				if err := pc.Clear(); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()

				parser := GetKotlinParser()
				tree, err := parser.ParseCtx(context.Background(), nil, src)
				if err != nil {
					b.Fatal(err)
				}
				flat := flattenTree(tree.RootNode())
				PutKotlinParser(parser)
				// Bypass the size gate so small buckets still exercise
				// the write path and we can see the cost we *would* pay
				// if the threshold were lowered.
				if err := pc.saveEntry(contentHashForBench(src), flat); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkParseCacheSweep_CacheHit(b *testing.B) {
	for _, size := range parseCacheSweepSizes {
		size := size
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			repo := b.TempDir()
			pc, err := NewParseCache(repo)
			if err != nil {
				b.Fatalf("NewParseCache: %v", err)
			}
			src := []byte(buildKotlinSourceAtLeast(size))
			// Seed the cache with a real parse result.
			parser := GetKotlinParser()
			tree, err := parser.ParseCtx(context.Background(), nil, src)
			if err != nil {
				b.Fatal(err)
			}
			flat := flattenTree(tree.RootNode())
			PutKotlinParser(parser)
			hash := contentHashForBench(src)
			if err := pc.saveEntry(hash, flat); err != nil {
				b.Fatalf("seed: %v", err)
			}
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got, ok := pc.loadByHash(hash)
				if !ok || got == nil {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func contentHashForBench(src []byte) string {
	// Avoid a dependency on hashutil.Memo in the hot loop — the
	// benchmark only needs a stable key.
	return fmt.Sprintf("bench-%d-%x", len(src), simpleChecksum(src))
}

func simpleChecksum(b []byte) uint64 {
	var h uint64 = 1469598103934665603
	for _, c := range b {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}
