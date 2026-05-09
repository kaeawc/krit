package oracle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildOraclePack_PanicsOnUnsortedItems guards the determinism
// contract: any caller that forgets to sort by key now triggers a
// loud failure at the producer rather than silently emitting
// non-deterministic bytes. Regression for #25.
func TestBuildOraclePack_PanicsOnUnsortedItems(t *testing.T) {
	items := []oraclePackItem{
		{key: "ffff", data: []byte("z"), crc: crc32.ChecksumIEEE([]byte("z"))},
		{key: "aaaa", data: []byte("a"), crc: crc32.ChecksumIEEE([]byte("a"))},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unsorted items")
		}
	}()
	_ = buildOraclePack(items)
}

// TestBuildOraclePack_DeterministicBytes asserts that buildOraclePack
// produces byte-identical output for inputs that compare equal, when
// the canonical sort is applied.
func TestBuildOraclePack_DeterministicBytes(t *testing.T) {
	items := func() []oraclePackItem {
		// Build via map iteration to pick up Go's randomized order,
		// mirroring how production callers in putMany construct items.
		latest := map[string][]byte{}
		for i := 0; i < 32; i++ {
			key := fmt.Sprintf("%064x", i*7919)
			latest[key] = []byte(fmt.Sprintf("payload-%03d", i))
		}
		out := make([]oraclePackItem, 0, len(latest))
		for k, blob := range latest {
			out = append(out, oraclePackItem{
				key: k, data: blob, crc: crc32.ChecksumIEEE(blob),
			})
		}
		sortOraclePackItems(out)
		return out
	}

	first := buildOraclePack(items())
	for i := 0; i < 200; i++ {
		got := buildOraclePack(items())
		if !bytes.Equal(first, got) {
			t.Fatalf("iter %d: byte-different pack output", i)
		}
	}

	// Spot-check via sha256 so failures print a stable summary.
	want := hex.EncodeToString(sha256.New().Sum(first))
	got := hex.EncodeToString(sha256.New().Sum(buildOraclePack(items())))
	if want != got {
		t.Fatalf("sha256 mismatch:\n  want %s\n  got  %s", want, got)
	}
}

// TestOraclePackHandle_PutMany_DeterministicBytes drives the same
// determinism contract through the production putMany path, which is
// the actual non-determinism source called out in #25 (map iteration
// over `existingIndex` and `latest`).
func TestOraclePackHandle_PutMany_DeterministicBytes(t *testing.T) {
	writes := makeWritesForDeterminismTest(40)

	first := writePackOnce(t, writes)
	firstHash := sha256OfFile(t, first)

	for i := 0; i < 50; i++ {
		path := writePackOnce(t, writes)
		if h := sha256OfFile(t, path); h != firstHash {
			t.Fatalf("iter %d: pack file hash differs\n  want %s\n  got  %s", i, firstHash, h)
		}
	}
}

// TestOraclePackHandle_PutMany_AppendIsDeterministic exercises the
// merge path: an existing pack with N entries, then a putMany with
// M new entries. Both the existing-index map range and the latest map
// range previously contributed non-determinism.
func TestOraclePackHandle_PutMany_AppendIsDeterministic(t *testing.T) {
	initial := makeWritesForDeterminismTest(20)
	follow := makeWritesForDeterminismTestOffset(20, 100)

	// Reference run: write initial, then follow, capture bytes.
	refDir := t.TempDir()
	refHandle := newPackHandle(refDir)
	if err := refHandle.putMany(initial); err != nil {
		t.Fatalf("initial putMany: %v", err)
	}
	if err := refHandle.putMany(follow); err != nil {
		t.Fatalf("follow putMany: %v", err)
	}
	refHash := sha256OfFile(t, refHandle.path)

	for i := 0; i < 50; i++ {
		dir := t.TempDir()
		h := newPackHandle(dir)
		if err := h.putMany(initial); err != nil {
			t.Fatalf("iter %d initial: %v", i, err)
		}
		if err := h.putMany(follow); err != nil {
			t.Fatalf("iter %d follow: %v", i, err)
		}
		if got := sha256OfFile(t, h.path); got != refHash {
			t.Fatalf("iter %d: pack hash differs\n  want %s\n  got  %s", i, refHash, got)
		}
	}
}

// --- helpers ---

func makeWritesForDeterminismTest(n int) []oracleEncodedEntryWrite {
	return makeWritesForDeterminismTestOffset(n, 0)
}

func makeWritesForDeterminismTestOffset(n, offset int) []oracleEncodedEntryWrite {
	writes := make([]oracleEncodedEntryWrite, 0, n)
	for i := 0; i < n; i++ {
		// Hashes within a single pack share their first byte (bucket).
		// Use a fixed prefix so all writes route to the same pack.
		hash := fmt.Sprintf("ab%062x", (offset+i)*1000003)
		blob := []byte(fmt.Sprintf("payload-%05d", offset+i))
		writes = append(writes, oracleEncodedEntryWrite{hash: hash, data: blob})
	}
	return writes
}

func newPackHandle(dir string) *oraclePackHandle {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	return &oraclePackHandle{path: filepath.Join(dir, "ab.pack")}
}

func writePackOnce(t *testing.T, writes []oracleEncodedEntryWrite) string {
	t.Helper()
	dir := t.TempDir()
	h := newPackHandle(dir)
	if err := h.putMany(writes); err != nil {
		t.Fatalf("putMany: %v", err)
	}
	return h.path
}

func sha256OfFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
