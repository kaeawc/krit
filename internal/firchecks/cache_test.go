package firchecks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCacheDir_CreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	dir, err := CacheDir(tmp)
	if err != nil {
		t.Fatalf("CacheDir failed: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("CacheDir did not create directory: %v", err)
	}
}

func TestCacheRoundtrip_EmptyFindings(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)

	entry := &FirCacheEntry{
		V:           FirCacheVersion,
		ContentHash: "abc123",
		FilePath:    "/src/Foo.kt",
		Findings:    nil,
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatalf("WriteCacheEntry failed: %v", err)
	}

	loaded, err := LoadCacheEntry(cacheDir, "/src/Foo.kt", "abc123")
	if err != nil {
		t.Fatalf("LoadCacheEntry failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected loaded entry, got nil")
	}
	if loaded.FilePath != "/src/Foo.kt" {
		t.Errorf("unexpected FilePath: %q", loaded.FilePath)
	}
}

func TestCacheRoundtrip_WithFindings(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)

	entry := &FirCacheEntry{
		V:           FirCacheVersion,
		ContentHash: "hash999",
		FilePath:    "/src/Bar.kt",
		Findings: []FirFinding{
			{Path: "/src/Bar.kt", Line: 10, Col: 5, Rule: "CollectInOnCreateWithoutLifecycle", Severity: "warning", Message: "test", Confidence: 1.0},
		},
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatalf("WriteCacheEntry failed: %v", err)
	}
	loaded, err := LoadCacheEntry(cacheDir, "/src/Bar.kt", "hash999")
	if err != nil {
		t.Fatalf("LoadCacheEntry failed: %v", err)
	}
	if len(loaded.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(loaded.Findings))
	}
	if loaded.Findings[0].Rule != "CollectInOnCreateWithoutLifecycle" {
		t.Errorf("unexpected rule: %q", loaded.Findings[0].Rule)
	}
}

func TestLoadCacheEntry_MissOnMissing(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)

	entry, err := LoadCacheEntry(cacheDir, "/src/Missing.kt", "nonexistent")
	if err != nil {
		t.Fatalf("expected nil error on miss, got %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry on miss, got %+v", entry)
	}
}

func TestLoadCacheEntry_VersionMismatchReturnsMiss(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)

	entry := &FirCacheEntry{
		V:           FirCacheVersion + 1, // wrong version
		ContentHash: "stale123",
		FilePath:    "/src/Stale.kt",
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatalf("WriteCacheEntry failed: %v", err)
	}
	loaded, err := LoadCacheEntry(cacheDir, "/src/Stale.kt", "stale123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected version-mismatch to be treated as miss, got %+v", loaded)
	}
}

func TestClassifyFiles_HitAndMiss(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)

	// Write a real .kt file with known content
	ktFile := tmp + "/Test.kt"
	if err := os.WriteFile(ktFile, []byte("fun main() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	hash, err := ContentHash(ktFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := &FirCacheEntry{
		V:           FirCacheVersion,
		ContentHash: hash,
		FilePath:    ktFile,
		Findings:    nil,
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatal(err)
	}

	otherFile := tmp + "/Other.kt" // not in cache
	if err := os.WriteFile(otherFile, []byte("class Other"), 0644); err != nil {
		t.Fatal(err)
	}

	hits, misses := ClassifyFiles(cacheDir, []string{ktFile, otherFile})
	if len(hits) != 1 {
		t.Errorf("expected 1 hit, got %d", len(hits))
	}
	if len(misses) != 1 {
		t.Errorf("expected 1 miss, got %d", len(misses))
	}
	if misses[0] != otherFile {
		t.Errorf("expected miss to be %q, got %q", otherFile, misses[0])
	}
}

func TestClassifyFilesForFingerprint_MissesWhenClasspathJarChanges(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)

	ktFile := tmp + "/Test.kt"
	if err := os.WriteFile(ktFile, []byte("fun main() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	jarFile := tmp + "/dep.jar"
	if err := os.WriteFile(jarFile, []byte("first jar"), 0644); err != nil {
		t.Fatal(err)
	}
	fp := ClasspathFingerprint([]string{jarFile})
	hash, err := ContentHash(ktFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := &FirCacheEntry{
		V:                  FirCacheVersion,
		ContentHash:        hash,
		FilePath:           ktFile,
		ClosureFingerprint: fp,
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatal(err)
	}

	hits, misses := ClassifyFilesForFingerprint(cacheDir, []string{ktFile}, fp)
	if len(hits) != 1 || len(misses) != 0 {
		t.Fatalf("expected initial fingerprint hit, hits=%d misses=%d", len(hits), len(misses))
	}
	if err := os.WriteFile(jarFile, []byte("second jar"), 0644); err != nil {
		t.Fatal(err)
	}
	changed := ClasspathFingerprint([]string{jarFile})
	if changed == fp {
		t.Fatal("classpath fingerprint did not change after jar content changed")
	}
	hits, misses = ClassifyFilesForFingerprint(cacheDir, []string{ktFile}, changed)
	if len(hits) != 0 || len(misses) != 1 || misses[0] != ktFile {
		t.Fatalf("expected jar content change to invalidate cache, hits=%d misses=%v", len(hits), misses)
	}
}

func TestCachePoisonEntry(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)

	entry := &FirCacheEntry{
		V:           FirCacheVersion,
		ContentHash: "crash1",
		FilePath:    "/src/Bad.kt",
		Crashed:     true,
		CrashError:  "FIR crash: StackOverflowError",
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCacheEntry(cacheDir, "/src/Bad.kt", "crash1")
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Crashed {
		t.Error("expected poison entry to have Crashed=true")
	}
	if loaded.CrashError == "" {
		t.Error("expected non-empty CrashError")
	}
}

func TestCacheMissWhenFirJarIdentityChanges(t *testing.T) {
	dir := t.TempDir()
	cacheDir, _ := CacheDir(dir)
	source := dir + "/A.kt"
	jar := dir + "/krit-fir.jar"
	if err := os.WriteFile(source, []byte("class A"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar, []byte("jar-one"), 0600); err != nil {
		t.Fatal(err)
	}
	first := FirInvocationFingerprint(nil, jar, []string{"A"}, nil, FileFacts{})
	WriteFreshEntriesForFingerprint(cacheDir, []string{source}, &CheckResponse{}, first)
	if hits, _ := ClassifyFilesForFingerprint(cacheDir, []string{source}, first); len(hits) != 1 {
		t.Fatal("expected initial hit")
	}
	if err := os.WriteFile(jar, []byte("jar-two-larger"), 0600); err != nil {
		t.Fatal(err)
	}
	second := FirInvocationFingerprint(nil, jar, []string{"A"}, nil, FileFacts{})
	if first == second {
		t.Fatal("jar fingerprint did not change")
	}
	if hits, misses := ClassifyFilesForFingerprint(cacheDir, []string{source}, second); len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("hits=%d misses=%d", len(hits), len(misses))
	}
}

func TestCacheMissWhenFirEnabledRulesChange(t *testing.T) {
	dir := t.TempDir()
	cacheDir, _ := CacheDir(dir)
	source := dir + "/A.kt"
	jar := dir + "/krit-fir.jar"
	if err := os.WriteFile(source, []byte("class A"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar, []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}
	first := FirInvocationFingerprint(nil, jar, []string{"A"}, nil, FileFacts{})
	WriteFreshEntriesForFingerprint(cacheDir, []string{source}, &CheckResponse{}, first)
	second := FirInvocationFingerprint(nil, jar, []string{"B"}, nil, FileFacts{})
	if first == second {
		t.Fatal("rule fingerprint did not change")
	}
	if hits, misses := ClassifyFilesForFingerprint(cacheDir, []string{source}, second); len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("hits=%d misses=%d", len(hits), len(misses))
	}
}

// Entries are per path: two files with identical content can have different
// verdicts (one compiles, one does not), and must not share an entry.
func TestFreshEntriesForIdenticalContentStayPerPath(t *testing.T) {
	dir := t.TempDir()
	clean := filepath.Join(dir, "a", "Same.kt")
	broken := filepath.Join(dir, "b", "Same.kt")
	for _, p := range []string{clean, broken} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("fun same() = helper()\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cacheDir, _ := CacheDir(t.TempDir())
	resp := &CheckResponse{ErrorFiles: map[string]string{broken: "Unresolved reference 'helper'."}}
	if n := WriteFreshEntriesForFingerprint(cacheDir, []string{broken, clean}, resp, "fp"); n != 2 {
		t.Fatalf("wrote %d entries, want 2", n)
	}
	hits, misses := ClassifyFilesForFingerprint(cacheDir, []string{clean, broken}, "fp")
	if len(hits) != 2 || len(misses) != 0 {
		t.Fatalf("hits=%d misses=%v, want both files to hit", len(hits), misses)
	}
	got := assembleFromCache(hits)
	if want := map[string]string{broken: "Unresolved reference 'helper'."}; !reflect.DeepEqual(got.ErrorFiles, want) {
		t.Fatalf("ErrorFiles = %v, want %v", got.ErrorFiles, want)
	}
}
