package firchecks

import (
	"os"
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

	loaded, err := LoadCacheEntry(cacheDir, "abc123")
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
			{Path: "/src/Bar.kt", Line: 10, Col: 5, Rule: "FLOW_COLLECT_IN_ON_CREATE", Severity: "warning", Message: "test", Confidence: 1.0},
		},
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatalf("WriteCacheEntry failed: %v", err)
	}
	loaded, err := LoadCacheEntry(cacheDir, "hash999")
	if err != nil {
		t.Fatalf("LoadCacheEntry failed: %v", err)
	}
	if len(loaded.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(loaded.Findings))
	}
	if loaded.Findings[0].Rule != "FLOW_COLLECT_IN_ON_CREATE" {
		t.Errorf("unexpected rule: %q", loaded.Findings[0].Rule)
	}
}

func TestLoadCacheEntry_MissOnMissing(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, _ := CacheDir(tmp)

	entry, err := LoadCacheEntry(cacheDir, "nonexistent")
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
	loaded, err := LoadCacheEntry(cacheDir, "stale123")
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
	loaded, err := LoadCacheEntry(cacheDir, "crash1")
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
