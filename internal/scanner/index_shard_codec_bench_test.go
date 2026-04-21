package scanner

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math/rand"
	"testing"
)

// buildSyntheticShard returns a fileShard sized roughly like a
// Signal-Android average: ~8 symbols and ~300 references drawn from a
// Zipf-ish name distribution so the intra-shard name table has real
// de-duplication to exploit.
func buildSyntheticShard(symCount, refCount int) *fileShard {
	rng := rand.New(rand.NewSource(0xC0FFEE))
	const vocab = 120
	names := make([]string, vocab)
	for i := range names {
		names[i] = fmt.Sprintf("identifier_%d_xyz", i)
	}
	kinds := []string{"function", "class", "property", "object", "interface"}
	vis := []string{"public", "private", "internal", "protected"}

	path := "/home/ci/workspace/Signal-Android/app/src/main/kotlin/org/thoughtcrime/securesms/database/MessageTable.kt"
	s := &fileShard{
		Path:        path,
		ContentHash: "deadbeef",
	}
	s.Symbols = make([]Symbol, symCount)
	for i := range s.Symbols {
		s.Symbols[i] = Symbol{
			Name:       names[rng.Intn(vocab)],
			Kind:       kinds[rng.Intn(len(kinds))],
			Visibility: vis[rng.Intn(len(vis))],
			File:       path,
			Line:       rng.Intn(5000),
			StartByte:  rng.Intn(200_000),
			EndByte:    rng.Intn(200_000) + 1,
			IsOverride: rng.Intn(4) == 0,
			IsTest:     rng.Intn(8) == 0,
		}
	}
	s.References = make([]Reference, refCount)
	// Zipf-ish: most refs hit the top ~20 identifiers.
	for i := range s.References {
		var idx int
		if rng.Intn(4) == 0 {
			idx = rng.Intn(vocab)
		} else {
			idx = rng.Intn(20)
		}
		s.References[i] = Reference{
			Name:      names[idx],
			File:      path,
			Line:      rng.Intn(5000),
			InComment: rng.Intn(32) == 0,
		}
	}
	return s
}

// BenchmarkShardEncodeV6 measures the combined columnar+zstd encode
// time per shard. On Signal-average (8 syms, 300 refs) this is what
// every cache miss pays on write.
func BenchmarkShardEncodeV6(b *testing.B) {
	s := buildSyntheticShard(8, 300)
	b.ResetTimer()
	var total int
	for i := 0; i < b.N; i++ {
		blob, err := encodeShardBlob(s)
		if err != nil {
			b.Fatalf("encode: %v", err)
		}
		total += len(blob)
	}
	b.ReportMetric(float64(total)/float64(b.N), "bytes/op")
}

// BenchmarkShardDecodeV6 measures the combined zstd+columnar decode
// time per shard. Cold crossFileAnalysis calls this once per hit.
func BenchmarkShardDecodeV6(b *testing.B) {
	s := buildSyntheticShard(8, 300)
	blob, err := encodeShardBlob(s)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := decodeShardBlob(blob, s.Path); err != nil {
			b.Fatalf("decode: %v", err)
		}
	}
}

// TestShardEncodingSizeComparison logs the on-disk blob size under
// three encoders (raw gob, v5 zstd+gob, v6 zstd+columnar) for a
// representative shard. Not an assertion — the goal is to keep the
// comparison visible in CI so regressions show up in test output.
func TestShardEncodingSizeComparison(t *testing.T) {
	shards := []struct {
		name    string
		symbols int
		refs    int
	}{
		{"small", 3, 50},
		{"average", 8, 300},
		{"large", 30, 1500},
	}
	fmt.Println("shard_size,gob,v5_zstd_gob,v6_zstd_columnar,v6_vs_gob,v6_vs_v5")
	for _, sh := range shards {
		s := buildSyntheticShard(sh.symbols, sh.refs)

		var gobBuf bytes.Buffer
		if err := gob.NewEncoder(&gobBuf).Encode(s); err != nil {
			t.Fatalf("gob encode: %v", err)
		}

		// v5 emulation: zstd over gob with File stripped.
		stripped := *s
		if len(s.Symbols) > 0 {
			stripped.Symbols = make([]Symbol, len(s.Symbols))
			copy(stripped.Symbols, s.Symbols)
			for i := range stripped.Symbols {
				stripped.Symbols[i].File = ""
			}
		}
		if len(s.References) > 0 {
			stripped.References = make([]Reference, len(s.References))
			copy(stripped.References, s.References)
			for i := range stripped.References {
				stripped.References[i].File = ""
			}
		}
		var v5Inner bytes.Buffer
		if err := gob.NewEncoder(&v5Inner).Encode(&stripped); err != nil {
			t.Fatalf("v5 gob encode: %v", err)
		}
		v5Blob := shardZstdEncoder.EncodeAll(v5Inner.Bytes(), nil)

		v6Blob, err := encodeShardBlob(s)
		if err != nil {
			t.Fatalf("v6 encode: %v", err)
		}

		t.Logf("%-8s gob=%d v5=%d v6=%d  v6/gob=%.2f%%  v6/v5=%.2f%%",
			sh.name,
			gobBuf.Len(), len(v5Blob), len(v6Blob),
			100*float64(len(v6Blob))/float64(gobBuf.Len()),
			100*float64(len(v6Blob))/float64(len(v5Blob)))

		// Correctness: v6 blob must round-trip to a shard whose
		// rebuilt File fields equal s.Path.
		got, err := decodeShardBlob(v6Blob, s.Path)
		if err != nil {
			t.Fatalf("v6 round-trip decode: %v", err)
		}
		if len(got.Symbols) != len(s.Symbols) || len(got.References) != len(s.References) {
			t.Fatalf("round-trip count mismatch")
		}
		for i, sym := range got.Symbols {
			if sym.File != s.Path {
				t.Errorf("sym[%d].File = %q, want %q", i, sym.File, s.Path)
			}
		}
	}
}
