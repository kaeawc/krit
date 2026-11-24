package oracle

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Oracle cache writes skip fsync, so after an OS crash or power loss a pack or
// facts record can be empty, cut short, zero-filled, or hold unrelated bytes.
// Each must read as a miss rather than a hit or a failed scan.

func tornOracleVariants(valid []byte) map[string][]byte {
	variants := map[string][]byte{
		"empty":       {},
		"zero-filled": make([]byte, len(valid)),
		"garbage":     bytes.Repeat([]byte("garbage!"), len(valid)/8+1),
	}
	for _, n := range []int{1, 4, len(valid) / 4, len(valid) / 2, len(valid) - 1} {
		if n > 0 && n < len(valid) {
			variants["truncated-"+strconv.Itoa(n)] = append([]byte(nil), valid[:n]...)
		}
	}
	return variants
}

func TestOraclePack_TornPackIsMiss(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	b := writeTempFile(t, tmp, "B.kt", "class B {}\n")
	a := writeTempFile(t, tmp, "A.kt", "class A : B()\n")
	hash, _ := ContentHash(a)
	fp, _ := closureFingerprint([]string{b}, nil)
	if err := WriteEntry(cacheDir, &CacheEntry{
		ContentHash: hash,
		FilePath:    a,
		FileResult:  &File{Package: "x"},
		PerFileDeps: map[string]*Class{},
		Closure:     CacheClosure{DepPaths: []string{b}, Fingerprint: fp},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	h, err := newOraclePackStore(cacheDir).packForHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(h.path)
	if err != nil {
		t.Fatalf("read pack: %v", err)
	}
	if hits, _ := ClassifyFiles(cacheDir, []string{a}); len(hits) != 1 {
		t.Fatalf("valid pack: hits = %d, want 1", len(hits))
	}
	for name, data := range tornOracleVariants(valid) {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(h.path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if entry, _ := LoadEntry(cacheDir, hash); entry != nil {
				t.Fatal("LoadEntry returned an entry from a damaged pack")
			}
			hits, misses := ClassifyFiles(cacheDir, []string{a})
			if len(hits) != 0 || len(misses) != 1 {
				t.Fatalf("damaged pack: hits=%d misses=%d, want 0/1", len(hits), len(misses))
			}
		})
	}
}

func TestTypesFacts_TornRecordIsMiss(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scope := storeScopeForToken(BackendFIR, "jar-x")
	if err := RecordTypesFacts(path, scope); err != nil {
		t.Fatal(err)
	}
	rec, ok := readTypesFacts(path)
	if !ok || !TypesJSONSatisfies(path, scope, rec.BuildToken) {
		t.Fatal("valid facts record was not accepted")
	}
	valid, err := os.ReadFile(typesFactsPath(path))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range tornOracleVariants(valid) {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(typesFactsPath(path), data, 0o644); err != nil {
				t.Fatal(err)
			}
			if TypesJSONSatisfies(path, scope, rec.BuildToken) {
				t.Fatal("damaged facts record vouched for types.json")
			}
		})
	}
}
