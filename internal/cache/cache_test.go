package cache

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func testFindingColumns(findings []scanner.Finding) scanner.FindingColumns {
	return scanner.CollectFindings(findings)
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, CacheFileName)

	original := &Cache{
		Version:  "1.0.0",
		RuleHash: "abc123",
		Files: map[string]FileEntry{
			"/src/a.kt": {Hash: "h1", ModTime: 1000, Size: 100},
			"/src/b.kt": {Hash: "h2", ModTime: 2000, Size: 200, Columns: testFindingColumns([]scanner.Finding{
				{File: "/src/b.kt", Line: 5, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long"},
			})},
		},
	}

	if err := original.Save(cachePath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if !strings.Contains(string(raw), `"columns"`) {
		t.Fatalf("expected cache file to persist columnar findings, got %s", raw)
	}
	if strings.Contains(string(raw), `"findings"`) {
		t.Fatalf("expected cache file to omit legacy findings field, got %s", raw)
	}

	loaded := Load(cachePath)
	if loaded.Version != original.Version {
		t.Errorf("expected version=%s, got %s", original.Version, loaded.Version)
	}
	if loaded.RuleHash != original.RuleHash {
		t.Errorf("expected ruleHash=%s, got %s", original.RuleHash, loaded.RuleHash)
	}
	if len(loaded.Files) != 2 {
		t.Errorf("expected 2 file entries, got %d", len(loaded.Files))
	}
	entry, ok := loaded.Files["/src/b.kt"]
	if !ok {
		t.Fatal("expected /src/b.kt in loaded cache")
	}
	if entry.Columns.Len() != 1 {
		t.Errorf("expected 1 cached row for b.kt, got %d", entry.Columns.Len())
	}
}

func TestCacheInvalidation_RuleHashChange(t *testing.T) {
	dir := t.TempDir()

	// Create a real file so NeedsReanalysis can stat it
	filePath := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(filePath, []byte("fun main() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	absPath, _ := filepath.Abs(filePath)

	c := &Cache{
		RuleHash: "oldhash",
		Files: map[string]FileEntry{
			absPath: {Hash: "h1", ModTime: 1000, Size: 14},
		},
	}

	// Different rule hash should cause all misses
	result := c.CheckFiles([]string{filePath}, "newhash")
	if result.TotalCached != 0 {
		t.Errorf("expected 0 cached with different ruleHash, got %d", result.TotalCached)
	}
}

func TestCheckFiles_HitAndMiss(t *testing.T) {
	dir := t.TempDir()

	// Create two files
	fileA := filepath.Join(dir, "a.kt")
	fileB := filepath.Join(dir, "b.kt")
	if err := os.WriteFile(fileA, []byte("fun a() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("fun b() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	absA, _ := filepath.Abs(fileA)
	infoA, _ := os.Stat(fileA)

	c := &Cache{
		RuleHash: "samehash",
		Files: map[string]FileEntry{
			absA: {
				Hash:    ComputeFileHash(fileA),
				ModTime: infoA.ModTime().UnixMilli(),
				Size:    infoA.Size(),
				Columns: testFindingColumns([]scanner.Finding{
					{File: fileA, Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "X", Message: "m"},
				}),
			},
			// fileB not in cache
		},
	}

	result := c.CheckFiles([]string{fileA, fileB}, "samehash")
	if result.TotalCached != 1 {
		t.Errorf("expected 1 cached hit, got %d", result.TotalCached)
	}
	if result.TotalFiles != 2 {
		t.Errorf("expected 2 total files, got %d", result.TotalFiles)
	}
	if !result.CachedPaths[fileA] {
		t.Error("expected fileA to be a cache hit")
	}
	if result.CachedPaths[fileB] {
		t.Error("expected fileB to be a cache miss")
	}
	if result.CachedColumns.Len() != 1 {
		t.Errorf("expected 1 cached finding, got %d", result.CachedColumns.Len())
	}
	if got := result.CachedColumns.Findings(); !reflect.DeepEqual(got, []scanner.Finding{
		{File: fileA, Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "X", Message: "m"},
	}) {
		t.Fatalf("cached columns mismatch:\nwant: %#v\ngot:  %#v", []scanner.Finding{
			{File: fileA, Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "X", Message: "m"},
		}, got)
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "sub", CacheFileName)

	c := &Cache{
		Version:  "1.0.0",
		RuleHash: "xyz",
		Files:    map[string]FileEntry{},
	}

	if err := c.Save(cachePath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists and is loadable
	loaded := Load(cachePath)
	if loaded.RuleHash != "xyz" {
		t.Errorf("expected ruleHash=xyz, got %s", loaded.RuleHash)
	}

	// Verify no temp files are left
	entries, err := os.ReadDir(filepath.Dir(cachePath))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestPrune_RemovesStaleEntries(t *testing.T) {
	dir := t.TempDir()

	// Create one real file
	realFile := filepath.Join(dir, "exists.kt")
	if err := os.WriteFile(realFile, []byte("fun x() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	absReal, _ := filepath.Abs(realFile)

	c := &Cache{
		Files: map[string]FileEntry{
			absReal:                       {Hash: "h1", ModTime: 1000, Size: 11},
			filepath.Join(dir, "gone.kt"): {Hash: "h2", ModTime: 2000, Size: 20},
		},
	}

	c.Prune()

	if _, ok := c.Files[absReal]; !ok {
		t.Error("expected existing file to remain in cache")
	}
	if _, ok := c.Files[filepath.Join(dir, "gone.kt")]; ok {
		t.Error("expected deleted file to be pruned from cache")
	}
}

func TestComputeConfigHash_DeterministicWithSameInput(t *testing.T) {
	rules := []string{"MaxLineLength", "FunctionNaming", "WildcardImport"}

	hash1 := ComputeConfigHash(rules, nil, false)
	hash2 := ComputeConfigHash(rules, nil, false)

	if hash1 != hash2 {
		t.Errorf("same inputs should produce same hash: %s != %s", hash1, hash2)
	}

	// Order shouldn't matter
	reversed := []string{"WildcardImport", "FunctionNaming", "MaxLineLength"}
	hash3 := ComputeConfigHash(reversed, nil, false)
	if hash1 != hash3 {
		t.Errorf("rule order should not matter: %s != %s", hash1, hash3)
	}
}

func TestComputeConfigHash_DifferentWithDifferentInput(t *testing.T) {
	rulesA := []string{"MaxLineLength", "FunctionNaming"}
	rulesB := []string{"MaxLineLength", "WildcardImport"}

	hashA := ComputeConfigHash(rulesA, nil, false)
	hashB := ComputeConfigHash(rulesB, nil, false)

	if hashA == hashB {
		t.Errorf("different rules should produce different hashes: %s == %s", hashA, hashB)
	}

	// Same rules but different editorconfig flag
	hashC := ComputeConfigHash(rulesA, nil, true)
	if hashA == hashC {
		t.Errorf("different editorconfig flag should produce different hash: %s == %s", hashA, hashC)
	}
}

func TestComputeFileHash_SameContent(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.kt")
	fileB := filepath.Join(dir, "b.kt")

	content := []byte("fun main() { println(\"hello\") }")
	if err := os.WriteFile(fileA, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, content, 0644); err != nil {
		t.Fatal(err)
	}

	hashA := ComputeFileHash(fileA)
	hashB := ComputeFileHash(fileB)

	if hashA == "" {
		t.Fatal("expected non-empty hash for existing file")
	}
	if hashA != hashB {
		t.Errorf("same content should produce same hash: %s != %s", hashA, hashB)
	}
}

func TestComputeFileHash_DifferentContent(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.kt")
	fileB := filepath.Join(dir, "b.kt")

	if err := os.WriteFile(fileA, []byte("fun a() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("fun b() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	hashA := ComputeFileHash(fileA)
	hashB := ComputeFileHash(fileB)

	if hashA == hashB {
		t.Errorf("different content should produce different hash: %s == %s", hashA, hashB)
	}
}

func TestComputeFileHash_NonexistentFile(t *testing.T) {
	hash := ComputeFileHash("/nonexistent/path/file.kt")
	if hash != "" {
		t.Errorf("expected empty hash for nonexistent file, got %s", hash)
	}
}

func TestCacheFilePath_DifferentScanPaths(t *testing.T) {
	cacheDir := "/tmp/krit-cache"

	pathA := CacheFilePath(cacheDir, []string{"/project/src"})
	pathB := CacheFilePath(cacheDir, []string{"/project/test"})

	if pathA == pathB {
		t.Errorf("different scan paths should produce different cache file paths: %s == %s", pathA, pathB)
	}
	if filepath.Dir(pathA) != cacheDir {
		t.Errorf("cache file should be in cacheDir, got %s", filepath.Dir(pathA))
	}
}

func TestCacheFilePath_EmptyCacheDir(t *testing.T) {
	path := CacheFilePath("", []string{"/project/src"})
	if path != "" {
		t.Errorf("expected empty string when cacheDir is empty, got %s", path)
	}
}

func TestCacheFilePath_NoScanPaths(t *testing.T) {
	cacheDir := "/tmp/krit-cache"
	path := CacheFilePath(cacheDir, nil)
	expected := filepath.Join(cacheDir, CacheFileName)
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestCacheFilePath_Deterministic(t *testing.T) {
	cacheDir := "/tmp/krit-cache"
	paths := []string{"/project/src", "/project/test"}

	path1 := CacheFilePath(cacheDir, paths)
	path2 := CacheFilePath(cacheDir, paths)

	if path1 != path2 {
		t.Errorf("same inputs should produce same cache path: %s != %s", path1, path2)
	}
}

func TestCacheFilePath_OrderIndependent(t *testing.T) {
	cacheDir := "/tmp/krit-cache"

	pathA := CacheFilePath(cacheDir, []string{"/project/src", "/project/test"})
	pathB := CacheFilePath(cacheDir, []string{"/project/test", "/project/src"})

	if pathA != pathB {
		t.Errorf("scan path order should not matter: %s != %s", pathA, pathB)
	}
}

func TestLoadSaveRoundTrip_WithFindings(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "test.cache")

	findings := []scanner.Finding{
		{File: "/src/a.kt", Line: 10, Col: 5, Severity: "error", RuleSet: "bugs", Rule: "NullDeref", Message: "potential null"},
		{File: "/src/a.kt", Line: 20, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "line too long"},
	}

	original := &Cache{
		Version:   "2.0.0",
		RuleHash:  "hash123",
		ScanPaths: []string{"/project/src"},
		Files: map[string]FileEntry{
			"/src/a.kt": {Hash: "abc", ModTime: 5000, Size: 500, Columns: testFindingColumns(findings)},
		},
	}

	if err := original.Save(cachePath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := Load(cachePath)

	if loaded.Version != "2.0.0" {
		t.Errorf("version mismatch: got %s", loaded.Version)
	}
	if loaded.RuleHash != "hash123" {
		t.Errorf("ruleHash mismatch: got %s", loaded.RuleHash)
	}
	if len(loaded.ScanPaths) != 1 || loaded.ScanPaths[0] != "/project/src" {
		t.Errorf("scanPaths mismatch: got %v", loaded.ScanPaths)
	}

	entry, ok := loaded.Files["/src/a.kt"]
	if !ok {
		t.Fatal("expected /src/a.kt in cache")
	}
	if got := entry.Columns.Findings(); !reflect.DeepEqual(got, findings) {
		t.Fatalf("loaded findings mismatch:\nwant: %#v\ngot:  %#v", findings, got)
	}
}

func TestLoad_LegacyFindingsFieldHydratesColumns(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "legacy.cache")

	raw := `{
  "version": "1.0.0",
  "ruleHash": "legacy",
  "files": {
    "/src/legacy.kt": {
      "hash": "h1",
      "modTime": 1000,
      "size": 42,
      "findings": [
        {
          "file": "/src/legacy.kt",
          "line": 3,
          "col": 1,
          "ruleSet": "style",
          "rule": "LegacyRule",
          "severity": "warning",
          "message": "legacy cache row"
        }
      ]
    }
  }
}`
	if err := os.WriteFile(cachePath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded := Load(cachePath)
	entry := loaded.Files["/src/legacy.kt"]

	if entry.Columns.Len() != 1 {
		t.Fatalf("expected legacy findings to hydrate 1 cached row, got %d", entry.Columns.Len())
	}
	if got := entry.Columns.Findings(); !reflect.DeepEqual(got, []scanner.Finding{{
		File:     "/src/legacy.kt",
		Line:     3,
		Col:      1,
		RuleSet:  "style",
		Rule:     "LegacyRule",
		Severity: "warning",
		Message:  "legacy cache row",
	}}) {
		t.Fatalf("legacy findings mismatch:\nwant: %#v\ngot:  %#v", []scanner.Finding{{
			File:     "/src/legacy.kt",
			Line:     3,
			Col:      1,
			RuleSet:  "style",
			Rule:     "LegacyRule",
			Severity: "warning",
			Message:  "legacy cache row",
		}}, got)
	}
}

func TestLoad_LegacyFindingsFieldRewritesAsColumnsOnly(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "legacy.cache")
	rewrittenPath := filepath.Join(dir, "rewritten.cache")

	raw := `{
  "version": "1.0.0",
  "ruleHash": "legacy",
  "files": {
    "/src/legacy.kt": {
      "hash": "h1",
      "modTime": 1000,
      "size": 42,
      "findings": [
        {
          "file": "/src/legacy.kt",
          "line": 3,
          "col": 1,
          "ruleSet": "style",
          "rule": "LegacyRule",
          "severity": "warning",
          "message": "legacy cache row"
        }
      ]
    }
  }
}`
	if err := os.WriteFile(legacyPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded := Load(legacyPath)
	if err := loaded.Save(rewrittenPath); err != nil {
		t.Fatalf("rewrite cache: %v", err)
	}

	rewritten, err := os.ReadFile(rewrittenPath)
	if err != nil {
		t.Fatalf("read rewritten cache: %v", err)
	}
	if !strings.Contains(string(rewritten), `"columns"`) {
		t.Fatalf("expected rewritten cache to persist columns, got %s", rewritten)
	}
	if strings.Contains(string(rewritten), `"findings"`) {
		t.Fatalf("expected rewritten cache to omit legacy findings field, got %s", rewritten)
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	c := Load("/nonexistent/cache/file")
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	if c.Files == nil {
		t.Fatal("expected non-nil Files map")
	}
	if len(c.Files) != 0 {
		t.Errorf("expected empty Files map, got %d entries", len(c.Files))
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "bad.cache")
	if err := os.WriteFile(cachePath, []byte("not json at all {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	c := Load(cachePath)
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	if c.Files == nil {
		t.Fatal("expected non-nil Files map for invalid JSON")
	}
}

func TestNeedsReanalysis_MatchingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "stable.kt")
	content := []byte("fun stable() {}")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(filePath)
	entry := FileEntry{
		Hash:    ComputeFileHash(filePath),
		ModTime: info.ModTime().UnixMilli(),
		Size:    info.Size(),
	}

	if NeedsReanalysis(filePath, entry) {
		t.Error("file with matching hash/modtime/size should not need reanalysis")
	}
}

func TestNeedsReanalysis_DifferentHash(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "changed.kt")
	if err := os.WriteFile(filePath, []byte("fun original() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Record entry with old modtime so fast-path doesn't match
	entry := FileEntry{
		Hash:    "oldhash",
		ModTime: 0, // forces slow path
		Size:    0,
	}

	if !NeedsReanalysis(filePath, entry) {
		t.Error("file with different hash should need reanalysis")
	}
}

func TestNeedsReanalysis_NonexistentFile(t *testing.T) {
	entry := FileEntry{Hash: "abc", ModTime: 1000, Size: 100}
	if !NeedsReanalysis("/nonexistent/file.kt", entry) {
		t.Error("nonexistent file should need reanalysis")
	}
}

func TestClear_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "to-delete.cache")

	// Create a cache file
	c := &Cache{Version: "1.0", RuleHash: "x", Files: map[string]FileEntry{}}
	if err := c.Save(cachePath); err != nil {
		t.Fatal(err)
	}

	// Verify it exists
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file should exist before Clear: %v", err)
	}

	if err := Clear(cachePath); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not exist after Clear")
	}
}

func TestClear_NonexistentFile(t *testing.T) {
	err := Clear("/nonexistent/path/cache.json")
	if err != nil {
		t.Errorf("Clear should not error on nonexistent file, got: %v", err)
	}
}

func TestClearSharedCache(t *testing.T) {
	dir := t.TempDir()

	// Create some cache files and a non-cache file
	os.WriteFile(filepath.Join(dir, "krit-abc123.cache"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "krit-def456.cache"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("keep"), 0644)

	if err := ClearSharedCache(dir); err != nil {
		t.Fatalf("ClearSharedCache failed: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "other.txt" {
			t.Errorf("expected only other.txt to remain, found %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file remaining, got %d", len(entries))
	}
}

func TestClearSharedCache_NonexistentDir(t *testing.T) {
	err := ClearSharedCache("/nonexistent/dir")
	if err != nil {
		t.Errorf("ClearSharedCache should not error on nonexistent dir, got: %v", err)
	}
}

func TestResolveCacheDir_CustomDir(t *testing.T) {
	dir, filePath := ResolveCacheDir("/custom/cache", []string{"/project/src"})
	if dir != "/custom/cache" {
		t.Errorf("expected custom dir, got %s", dir)
	}
	if filepath.Dir(filePath) != "/custom/cache" {
		t.Errorf("expected file in custom dir, got %s", filePath)
	}
}

func TestResolveCacheDir_FallbackToScanPath(t *testing.T) {
	tmpDir := t.TempDir()
	dir, filePath := ResolveCacheDir("", []string{tmpDir})
	absDir, _ := filepath.Abs(tmpDir)
	if dir != absDir {
		t.Errorf("expected fallback to scan path dir %s, got %s", absDir, dir)
	}
	if filepath.Base(filePath) != CacheFileName {
		t.Errorf("expected cache filename %s, got %s", CacheFileName, filepath.Base(filePath))
	}
}

func TestResolveCacheDir_FileAsScanPath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "some.kt")
	if err := os.WriteFile(filePath, []byte("fun x() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	resolvedDir, cachePath := ResolveCacheDir("", []string{filePath})
	absDir, _ := filepath.Abs(dir)
	if resolvedDir != absDir {
		t.Errorf("expected dir=%s when scan path is a file, got %s", absDir, resolvedDir)
	}
	if filepath.Base(cachePath) != CacheFileName {
		t.Errorf("expected cache filename %s, got %s", CacheFileName, filepath.Base(cachePath))
	}
}

func TestUpdateEntry(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "entry.kt")
	content := []byte("fun entry() {}")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}
	absPath, _ := filepath.Abs(filePath)

	c := &Cache{Files: make(map[string]FileEntry)}
	findings := []scanner.Finding{
		{File: filePath, Line: 1, Col: 1, Severity: "warning", Rule: "TestRule", Message: "msg"},
	}

	c.UpdateEntry(filePath, findings)

	entry, ok := c.Files[absPath]
	if !ok {
		t.Fatal("expected entry to be added to cache")
	}
	if entry.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if entry.Size != int64(len(content)) {
		t.Errorf("expected size=%d, got %d", len(content), entry.Size)
	}
	if entry.Columns.Len() != 1 {
		t.Errorf("expected 1 cached row, got %d", entry.Columns.Len())
	}
}

func TestUpdateEntryColumns(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "entry.kt")
	content := []byte("fun entry() {}")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}
	absPath, _ := filepath.Abs(filePath)

	c := &Cache{Files: make(map[string]FileEntry)}
	findings := []scanner.Finding{
		{
			File:     filePath,
			Line:     1,
			Col:      1,
			Severity: "warning",
			RuleSet:  "style",
			Rule:     "TestRule",
			Message:  "msg",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "fixed()",
			},
			BinaryFix: &scanner.BinaryFix{
				Type:    scanner.BinaryFixCreateFile,
				Content: []byte("payload"),
			},
			Confidence: 0.83,
		},
	}
	columns := scanner.CollectFindings(findings)

	c.UpdateEntryColumns(filePath, &columns)

	entry, ok := c.Files[absPath]
	if !ok {
		t.Fatal("expected entry to be added to cache")
	}
	if entry.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if entry.Size != int64(len(content)) {
		t.Errorf("expected size=%d, got %d", len(content), entry.Size)
	}
	if got := entry.Columns.Findings(); !reflect.DeepEqual(got, findings) {
		t.Fatalf("cached findings mismatch:\nwant: %#v\ngot:  %#v", findings, got)
	}
}

func TestCheckFiles_CachedColumnsPreserveFixPayloads(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "fixable.kt")
	content := []byte("fun fixable() {}")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}
	absPath, _ := filepath.Abs(filePath)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}

	findings := []scanner.Finding{
		{
			File:     filePath,
			Line:     3,
			Col:      2,
			Severity: "warning",
			RuleSet:  "style",
			Rule:     "FixableRule",
			Message:  "replace me",
			Fix: &scanner.Fix{
				StartLine:   3,
				EndLine:     3,
				Replacement: "fixed()",
			},
			BinaryFix: &scanner.BinaryFix{
				Type:       scanner.BinaryFixCreateFile,
				TargetPath: filepath.Join(dir, "generated.txt"),
				Content:    []byte("payload"),
			},
			Confidence: 0.91,
		},
	}

	c := &Cache{
		RuleHash: "samehash",
		Files: map[string]FileEntry{
			absPath: {
				Hash:    ComputeFileHash(filePath),
				ModTime: info.ModTime().UnixMilli(),
				Size:    info.Size(),
				Columns: testFindingColumns(findings),
			},
		},
	}

	result := c.CheckFiles([]string{filePath}, "samehash")
	if result.CachedColumns.Len() != 1 {
		t.Fatalf("expected 1 cached row, got %d", result.CachedColumns.Len())
	}
	if got := result.CachedColumns.Findings(); !reflect.DeepEqual(got, findings) {
		t.Fatalf("cached column round-trip mismatch:\nwant: %#v\ngot:  %#v", findings, got)
	}
}

func TestComputeRuleHash_Deterministic(t *testing.T) {
	rules := []string{"RuleA", "RuleB", "RuleC"}
	hash1 := ComputeRuleHash(rules)
	hash2 := ComputeRuleHash(rules)
	if hash1 != hash2 {
		t.Errorf("same rules should produce same hash: %s != %s", hash1, hash2)
	}

	// Order should not matter
	reversed := []string{"RuleC", "RuleB", "RuleA"}
	hash3 := ComputeRuleHash(reversed)
	if hash1 != hash3 {
		t.Errorf("rule order should not matter: %s != %s", hash1, hash3)
	}
}

func TestComputeRuleHash_DifferentRules(t *testing.T) {
	hashA := ComputeRuleHash([]string{"RuleA", "RuleB"})
	hashB := ComputeRuleHash([]string{"RuleA", "RuleC"})
	if hashA == hashB {
		t.Errorf("different rules should produce different hashes: %s == %s", hashA, hashB)
	}
}

func TestSaveToDir_LoadFromDir_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &Cache{
		Version:  "1.0.0",
		RuleHash: "roundtrip",
		Files: map[string]FileEntry{
			"/src/test.kt": {Hash: "h1", ModTime: 3000, Size: 42, Columns: testFindingColumns([]scanner.Finding{
				{File: "/src/test.kt", Line: 7, Col: 2, Severity: "error", RuleSet: "bugs", Rule: "TestRule", Message: "test msg"},
			})},
		},
	}

	if err := original.SaveToDir(dir); err != nil {
		t.Fatalf("SaveToDir failed: %v", err)
	}

	// Verify file exists at expected path
	cachePath := filepath.Join(dir, CacheFileName)
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected cache file at %s: %v", cachePath, err)
	}

	loaded := LoadFromDir(dir)
	if loaded.Version != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", loaded.Version)
	}
	if loaded.RuleHash != "roundtrip" {
		t.Errorf("expected ruleHash=roundtrip, got %s", loaded.RuleHash)
	}
	if len(loaded.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(loaded.Files))
	}
	entry, ok := loaded.Files["/src/test.kt"]
	if !ok {
		t.Fatal("expected /src/test.kt in loaded cache")
	}
	if got := entry.Columns.Findings(); !reflect.DeepEqual(got, []scanner.Finding{
		{File: "/src/test.kt", Line: 7, Col: 2, Severity: "error", RuleSet: "bugs", Rule: "TestRule", Message: "test msg"},
	}) {
		t.Fatalf("loaded findings mismatch:\nwant: %#v\ngot:  %#v", []scanner.Finding{
			{File: "/src/test.kt", Line: 7, Col: 2, Severity: "error", RuleSet: "bugs", Rule: "TestRule", Message: "test msg"},
		}, got)
	}
}

func TestClearDir_RemovesCacheFile(t *testing.T) {
	dir := t.TempDir()

	c := &Cache{Version: "1.0", RuleHash: "clear", Files: map[string]FileEntry{}}
	if err := c.SaveToDir(dir); err != nil {
		t.Fatalf("SaveToDir failed: %v", err)
	}

	cachePath := filepath.Join(dir, CacheFileName)
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file should exist before ClearDir: %v", err)
	}

	if err := ClearDir(dir); err != nil {
		t.Fatalf("ClearDir failed: %v", err)
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not exist after ClearDir")
	}
}

func TestClearDir_NonexistentDir(t *testing.T) {
	err := ClearDir("/nonexistent/dir/for/cache")
	if err != nil {
		t.Errorf("ClearDir should not error on nonexistent dir, got: %v", err)
	}
}

func TestResolveCacheDir(t *testing.T) {
	t.Run("with cache-dir flag", func(t *testing.T) {
		dir, filePath := ResolveCacheDir("/tmp/krit-cache", []string{"src/main"})
		if dir != "/tmp/krit-cache" {
			t.Errorf("expected dir=/tmp/krit-cache, got %s", dir)
		}
		if filepath.Dir(filePath) != "/tmp/krit-cache" {
			t.Errorf("expected file in /tmp/krit-cache, got %s", filePath)
		}
	})

	t.Run("without cache-dir flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		dir, filePath := ResolveCacheDir("", []string{tmpDir})
		absDir, _ := filepath.Abs(tmpDir)
		if dir != absDir {
			t.Errorf("expected dir=%s, got %s", absDir, dir)
		}
		if filepath.Base(filePath) != CacheFileName {
			t.Errorf("expected file named %s, got %s", CacheFileName, filepath.Base(filePath))
		}
	})

	t.Run("no scan paths", func(t *testing.T) {
		dir, filePath := ResolveCacheDir("", nil)
		if dir != "." {
			t.Errorf("expected dir=., got %s", dir)
		}
		if filePath != filepath.Join(".", CacheFileName) {
			t.Errorf("expected filePath=%s, got %s", filepath.Join(".", CacheFileName), filePath)
		}
	})
}
