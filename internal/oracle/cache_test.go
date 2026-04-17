package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeTempFile writes content to a new file under dir and returns the
// absolute path. Test helper.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestContentHash_StableAcrossReads(t *testing.T) {
	tmp := t.TempDir()
	p := writeTempFile(t, tmp, "a.kt", "fun main() {}\n")
	h1, err := ContentHash(p)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	h2, err := ContentHash(p)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("content hash not stable: %s vs %s", h1, h2)
	}
	// Rewriting with the same content must yield the same hash.
	if err := os.WriteFile(p, []byte("fun main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h3, _ := ContentHash(p)
	if h3 != h1 {
		t.Fatalf("rewrite same content changed hash: %s vs %s", h3, h1)
	}
}

func TestClassifyFiles_FreshMiss_NoEntry(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, tmp, "A.kt", "class A {}\n")
	hits, misses := ClassifyFiles(cacheDir, []string{a})
	if len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("expected 0 hits / 1 miss, got hits=%d misses=%d", len(hits), len(misses))
	}
	if misses[0] != a {
		t.Fatalf("wrong miss path: %s", misses[0])
	}
}

func TestClassifyFiles_WarmHit(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// Two files: A depends on B.
	b := writeTempFile(t, tmp, "B.kt", "class B {}\n")
	a := writeTempFile(t, tmp, "A.kt", "class A : B()\n")

	hash, _ := ContentHash(a)
	fp, _ := closureFingerprint([]string{b}, nil)
	entry := &CacheEntry{
		ContentHash: hash,
		FilePath:    a,
		FileResult:  &OracleFile{Package: "x"},
		PerFileDeps: map[string]*OracleClass{},
		Closure:     CacheClosure{DepPaths: []string{b}, Fingerprint: fp},
	}
	if err := WriteEntry(cacheDir, entry); err != nil {
		t.Fatalf("write: %v", err)
	}

	hits, misses := ClassifyFiles(cacheDir, []string{a})
	if len(hits) != 1 || len(misses) != 0 {
		t.Fatalf("expected 1 hit / 0 miss, got hits=%d misses=%d", len(hits), len(misses))
	}
	if hits[0].FilePath != a {
		t.Fatalf("wrong hit path: %s", hits[0].FilePath)
	}
	if hits[0].FileResult == nil || hits[0].FileResult.Package != "x" {
		t.Fatalf("hit lost FileResult contents")
	}
}

func TestClassifyFiles_ClosureChanged_Miss(t *testing.T) {
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
		FileResult:  &OracleFile{Package: "x"},
		Closure:     CacheClosure{DepPaths: []string{b}, Fingerprint: fp},
	}); err != nil {
		t.Fatal(err)
	}
	// Mutate B so its hash changes, but leave A intact (A's content hash
	// still matches, so we want the cache to miss on closure only).
	if err := os.WriteFile(b, []byte("class B { fun x() {} }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits, misses := ClassifyFiles(cacheDir, []string{a})
	if len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("expected 0 hits / 1 miss after closure change, got hits=%d misses=%d", len(hits), len(misses))
	}
}

func TestClassifyFiles_CorruptEntry_Miss(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, tmp, "A.kt", "class A {}\n")
	hash, _ := ContentHash(a)

	// Write deliberately invalid JSON at the entry path.
	ep := entryPath(cacheDir, hash)
	if err := os.MkdirAll(filepath.Dir(ep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep, []byte("not json {"), 0o644); err != nil {
		t.Fatal(err)
	}

	hits, misses := ClassifyFiles(cacheDir, []string{a})
	if len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("expected 0 hits / 1 miss for corrupt entry, got hits=%d misses=%d", len(hits), len(misses))
	}
	// Corrupt file should have been best-effort deleted.
	if _, err := os.Stat(ep); !os.IsNotExist(err) {
		t.Fatalf("corrupt entry not deleted: %v", err)
	}
}

func TestClassifyFiles_VersionMismatch_Miss(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, tmp, "A.kt", "class A {}\n")
	hash, _ := ContentHash(a)

	// Hand-write an entry with a bogus version.
	bad := map[string]any{
		"v":            999,
		"content_hash": hash,
		"file_path":    a,
		"closure":      map[string]any{"dep_paths": []string{}, "fingerprint": ""},
	}
	data, _ := json.Marshal(bad)
	ep := entryPath(cacheDir, hash)
	_ = os.MkdirAll(filepath.Dir(ep), 0o755)
	if err := os.WriteFile(ep, data, 0o644); err != nil {
		t.Fatal(err)
	}

	hits, misses := ClassifyFiles(cacheDir, []string{a})
	if len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("expected 0 hits / 1 miss for version mismatch, got hits=%d misses=%d", len(hits), len(misses))
	}
}

func TestCacheDir_BumpNukes(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// Drop a file inside entries/.
	sentinel := filepath.Join(cacheDir, "entries", "sentinel")
	if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Manually lower the recorded version so the next CacheDir sees a
	// mismatch and nukes the entries subtree.
	_ = os.WriteFile(filepath.Join(cacheDir, "version"), []byte("0"), 0o644)
	if _, err := CacheDir(tmp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("sentinel survived version bump: %v", err)
	}
}

func TestWriteEntry_Atomic_ReadBack(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)
	entry := &CacheEntry{
		ContentHash: "deadbeef",
		FilePath:    "/tmp/X.kt",
		FileResult: &OracleFile{
			Package: "p",
			Declarations: []*OracleClass{
				{FQN: "p.X", Kind: "class"},
			},
		},
		PerFileDeps: map[string]*OracleClass{
			"java.lang.Object": {FQN: "java.lang.Object", Kind: "class"},
		},
		Closure: CacheClosure{DepPaths: []string{}, Fingerprint: "abc"},
	}
	if err := WriteEntry(cacheDir, entry); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadEntry(cacheDir, "deadbeef")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.FilePath != entry.FilePath {
		t.Fatalf("path roundtrip broken: %s vs %s", got.FilePath, entry.FilePath)
	}
	if got.FileResult == nil || len(got.FileResult.Declarations) != 1 {
		t.Fatalf("declarations roundtrip broken")
	}
	if got.PerFileDeps["java.lang.Object"] == nil {
		t.Fatalf("per-file deps roundtrip broken")
	}
}

func TestAssembleOracle_UnionsHitsAndFresh(t *testing.T) {
	hit := &CacheEntry{
		FilePath:   "/tmp/A.kt",
		FileResult: &OracleFile{Package: "a"},
		PerFileDeps: map[string]*OracleClass{
			"kotlin.Any":   {FQN: "kotlin.Any", Kind: "class"},
			"kotlin.Unit":  {FQN: "kotlin.Unit", Kind: "class"},
		},
	}
	fresh := &OracleData{
		Version:       1,
		KotlinVersion: "2.3.20",
		Files: map[string]*OracleFile{
			"/tmp/B.kt": {Package: "b"},
		},
		Dependencies: map[string]*OracleClass{
			"kotlin.Unit":   {FQN: "kotlin.Unit", Kind: "fresh-wins"},
			"kotlin.String": {FQN: "kotlin.String", Kind: "class"},
		},
	}
	out := AssembleOracle([]*CacheEntry{hit}, fresh)
	if out.Files["/tmp/A.kt"] == nil {
		t.Fatalf("hit file missing from assembled oracle")
	}
	if out.Files["/tmp/B.kt"] == nil {
		t.Fatalf("fresh file missing from assembled oracle")
	}
	if out.Dependencies["kotlin.Any"] == nil || out.Dependencies["kotlin.Any"].Kind != "class" {
		t.Fatalf("hit-only dep not preserved")
	}
	if out.Dependencies["kotlin.Unit"].Kind != "fresh-wins" {
		t.Fatalf("fresh did not override hit on conflict")
	}
	if out.Dependencies["kotlin.String"] == nil {
		t.Fatalf("fresh-only dep missing")
	}
	if out.KotlinVersion != "2.3.20" {
		t.Fatalf("kotlinVersion not propagated")
	}
}

func TestPoisonEntry_WriteReadClassify(t *testing.T) {
	tmp := t.TempDir()
	src := writeTempFile(t, tmp, "Poison.kt", "class P // boom\n")
	cache, err := CacheDir(tmp)
	if err != nil {
		t.Fatalf("cache dir: %v", err)
	}

	// Simulate what krit-types emits for a crashed file: no FileResult, no
	// dep tracking, only a Crashed marker in the CacheDepsFile.
	deps := &CacheDepsFile{
		Version:       1,
		Approximation: "symbol-resolved-sources",
		Files:         map[string]*CacheDepsEntry{},
		Crashed: map[string]string{
			src: "KotlinIllegalArgumentExceptionWithAttachments: FirPropertyImpl without source",
		},
	}
	fresh := &OracleData{
		Version:      1,
		Files:        map[string]*OracleFile{},
		Dependencies: map[string]*OracleClass{},
	}
	written, err := WriteFreshEntries(cache, fresh, deps)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if written != 1 {
		t.Fatalf("expected 1 poison entry written, got %d", written)
	}

	// Classify the same file — it must come back as a HIT via the poison
	// marker without re-analyzing.
	hits, misses := ClassifyFiles(cache, []string{src})
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit (poison), got %d; misses=%v", len(hits), misses)
	}
	if len(misses) != 0 {
		t.Fatalf("expected 0 misses for poison entry, got %d (%v)", len(misses), misses)
	}
	h := hits[0]
	if !h.Crashed {
		t.Fatalf("hit is not marked crashed")
	}
	if h.FileResult != nil {
		t.Fatalf("poison entry should have nil FileResult, got %+v", h.FileResult)
	}
	if h.CrashError == "" {
		t.Fatalf("poison entry missing CrashError")
	}

	// Assemble — the poison entry contributes nothing to the output.
	out := AssembleOracle(hits, nil)
	if len(out.Files) != 0 {
		t.Fatalf("poison entry should not populate Files, got %d", len(out.Files))
	}
	if len(out.Dependencies) != 0 {
		t.Fatalf("poison entry should not populate Dependencies, got %d", len(out.Dependencies))
	}

	// Content change invalidates the poison marker → new content means a
	// new hash, so ClassifyFiles produces a miss and the file gets re-tried.
	if err := os.WriteFile(src, []byte("class P2 // different content\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	hits2, misses2 := ClassifyFiles(cache, []string{src})
	if len(misses2) != 1 {
		t.Fatalf("content change should invalidate poison; got hits=%d misses=%d", len(hits2), len(misses2))
	}
}
