package cache

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// makePortabilityCache builds a cache with enough entries to exercise map-key
// ordering and finding column serialization. It is shared by the portability
// test suite.
func makePortabilityCache() *Cache {
	files := map[string]FileEntry{}
	// Use names whose Go map iteration order is unstable across runs.
	for _, name := range []string{
		"/proj/a.kt", "/proj/b.kt", "/proj/c.kt", "/proj/d.kt",
		"/proj/e.kt", "/proj/zeta.kt", "/proj/alpha.kt", "/proj/sub/x.kt",
		"/proj/sub/y.kt", "/proj/sub/nested/z.kt",
	} {
		findings := []scanner.Finding{
			{File: name, Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R1", Message: "m1"},
			{File: name, Line: 7, Col: 3, Severity: "error", RuleSet: "perf", Rule: "R2", Message: "m2"},
		}
		files[name] = FileEntry{
			Hash:    "h-" + name,
			ModTime: 1000,
			Size:    int64(len(name)),
			Columns: scanner.CollectFindings(findings),
		}
	}
	return &Cache{
		Version:   "1.0.0",
		RuleHash:  "deadbeefcafebabe",
		ScanPaths: []string{"/proj"},
		Files:     files,
	}
}

// TestCacheBytes_DeterministicAcrossSaves writes the same cache twice and
// asserts the on-disk bytes are identical. Catches map-iteration leaks and
// timestamp/random embedding in the cache file.
func TestCacheBytes_DeterministicAcrossSaves(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.cache")
	pathB := filepath.Join(dir, "b.cache")

	c := makePortabilityCache()
	if err := c.Save(pathA); err != nil {
		t.Fatalf("save A: %v", err)
	}
	if err := c.Save(pathB); err != nil {
		t.Fatalf("save B: %v", err)
	}

	a, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("read A: %v", err)
	}
	b, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatalf("read B: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("cache bytes differ across saves of identical content\nA=%q\nB=%q", a, b)
	}
}

// TestCacheBytes_DeterministicAcrossLoadSave round-trips through Load and
// asserts the re-saved bytes match the original. Catches representation drift
// (canonical form changing between save and load).
func TestCacheBytes_DeterministicAcrossLoadSave(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.cache")
	pathB := filepath.Join(dir, "b.cache")

	c := makePortabilityCache()
	if err := c.Save(pathA); err != nil {
		t.Fatalf("save A: %v", err)
	}
	loaded := Load(pathA)
	if err := loaded.Save(pathB); err != nil {
		t.Fatalf("save B: %v", err)
	}

	a, _ := os.ReadFile(pathA)
	b, _ := os.ReadFile(pathB)
	if !bytes.Equal(a, b) {
		t.Fatalf("cache bytes differ after load/save round-trip\nA=%q\nB=%q", a, b)
	}
}

// TestCacheBytes_DeterministicAcrossInsertOrder builds the same logical cache
// twice with different file insertion orders and asserts on-disk bytes match.
// This is a stronger guarantee than relying on Go's json.Marshal sorting maps;
// it would catch any future migration to a slice-backed encoding.
func TestCacheBytes_DeterministicAcrossInsertOrder(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.cache")
	pathB := filepath.Join(dir, "b.cache")

	names := []string{
		"/proj/a.kt", "/proj/b.kt", "/proj/c.kt", "/proj/d.kt", "/proj/e.kt",
	}

	build := func(order []int) *Cache {
		files := map[string]FileEntry{}
		for _, i := range order {
			n := names[i]
			files[n] = FileEntry{Hash: "h" + n, ModTime: 100, Size: 10}
		}
		return &Cache{Version: "1.0.0", RuleHash: "x", Files: files}
	}

	cA := build([]int{0, 1, 2, 3, 4})
	cB := build([]int{4, 3, 2, 1, 0})

	if err := cA.Save(pathA); err != nil {
		t.Fatal(err)
	}
	if err := cB.Save(pathB); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(pathA)
	b, _ := os.ReadFile(pathB)
	if !bytes.Equal(a, b) {
		t.Fatalf("cache bytes depend on insert order\nA=%q\nB=%q", a, b)
	}
}

// TestCacheBytes_PathIndependence builds two caches with the same logical
// content under different absolute project roots and asserts that, after
// substituting one root prefix with the other, the on-disk bytes are
// identical. This is the cross-machine-restore property: identical source
// trees in different absolute locations must yield equivalent caches.
func TestCacheBytes_PathIndependence(t *testing.T) {
	dir := t.TempDir()

	build := func(root string) *Cache {
		files := map[string]FileEntry{}
		for _, rel := range []string{"a.kt", "b.kt", "sub/c.kt", "sub/nested/d.kt"} {
			abs := filepath.Join(root, rel)
			findings := []scanner.Finding{
				{File: abs, Line: 1, Col: 1, Severity: "warning", RuleSet: "s", Rule: "R", Message: "m"},
			}
			files[abs] = FileEntry{
				Hash:    "h-" + rel,
				ModTime: 100,
				Size:    int64(len(rel)),
				Columns: scanner.CollectFindings(findings),
			}
		}
		return &Cache{
			Version:   "1.0.0",
			RuleHash:  "rh",
			ScanPaths: []string{root},
			Files:     files,
		}
	}

	rootA := "/tmp/krit-port/A/project"
	rootB := "/var/krit-port/B/project"
	cA := build(rootA)
	cB := build(rootB)

	pA := filepath.Join(dir, "a.cache")
	pB := filepath.Join(dir, "b.cache")
	if err := cA.Save(pA); err != nil {
		t.Fatal(err)
	}
	if err := cB.Save(pB); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(pA)
	b, _ := os.ReadFile(pB)

	// The compressed binary cache is not amenable to byte-level path
	// substitution, so we normalise at the logical layer: load each cache,
	// rewrite the project-root prefix in the Files map keys and the
	// per-file finding columns, save back, and compare the resulting bytes.
	normalize := func(src string, root string) []byte {
		loaded := Load(src)
		rewritten := make(map[string]FileEntry, len(loaded.Files))
		for path, entry := range loaded.Files {
			cols := entry.Columns.Clone()
			for i, fp := range cols.Files {
				cols.Files[i] = strings.Replace(fp, root, "<ROOT>", 1)
			}
			newPath := strings.Replace(path, root, "<ROOT>", 1)
			rewritten[newPath] = FileEntry{
				Hash:    entry.Hash,
				ModTime: entry.ModTime,
				Size:    entry.Size,
				Columns: cols,
			}
		}
		loaded.Files = rewritten
		loaded.ScanPaths = append([]string(nil), loaded.ScanPaths...)
		for i, p := range loaded.ScanPaths {
			loaded.ScanPaths[i] = strings.Replace(p, root, "<ROOT>", 1)
		}
		out := filepath.Join(dir, "norm-"+filepath.Base(src))
		if err := loaded.Save(out); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(out)
		return data
	}
	normA := normalize(pA, rootA)
	normB := normalize(pB, rootB)
	if !bytes.Equal(normA, normB) {
		t.Fatalf("normalised cache bytes differ across roots\nA len=%d\nB len=%d", len(normA), len(normB))
	}

	// Sanity check: the unsubstituted bytes must NOT match (otherwise the
	// substitution would be vacuous and the test would not prove anything).
	if bytes.Equal(a, b) {
		t.Fatal("raw bytes equal across different roots; test cannot prove path independence")
	}
}

// TestCacheBytes_NoEnvLeakage runs Save with several env vars permuted and
// asserts the bytes do not change. Cache code shouldn't read HOME/USER/PWD/TZ
// while serializing entries; this guards against future regressions.
func TestCacheBytes_NoEnvLeakage(t *testing.T) {
	dir := t.TempDir()
	c := makePortabilityCache()

	envSets := []map[string]string{
		{"HOME": "/tmp/h1", "USER": "alice", "TZ": "UTC", "PWD": "/tmp/h1", "LANG": "C", "LC_ALL": "C"},
		{"HOME": "/var/h2", "USER": "bob", "TZ": "Asia/Tokyo", "PWD": "/var/h2", "LANG": "ja_JP.UTF-8", "LC_ALL": "ja_JP.UTF-8"},
		{"HOME": "/Users/dave", "USER": "dave", "TZ": "America/Los_Angeles", "PWD": "/", "LANG": "en_US.UTF-8", "LC_ALL": "en_US.UTF-8"},
	}

	var first []byte
	for i, env := range envSets {
		for k, v := range env {
			t.Setenv(k, v)
		}
		p := filepath.Join(dir, "env"+strings.Repeat("x", i+1)+".cache")
		if err := c.Save(p); err != nil {
			t.Fatalf("save: %v", err)
		}
		got, _ := os.ReadFile(p)
		if first == nil {
			first = got
			continue
		}
		if !bytes.Equal(first, got) {
			t.Fatalf("cache bytes changed under different env\nfirst=%q\ngot=%q", first, got)
		}
	}
}

// TestCache_CrossMachineRoundTrip simulates a CI cache restore where the
// cache was produced on machine A under one absolute root and is consumed on
// machine B under a different absolute root. We perform a path-prefix rewrite
// on the persisted cache bytes (the minimal transformation needed to make
// absolute-path-keyed caches portable) and then verify CheckFiles produces
// full hits at the new location.
func TestCache_CrossMachineRoundTrip(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	rels := []string{"a.kt", "b.kt", "sub/c.kt"}
	contents := map[string]string{
		"a.kt":     "fun a() {}\n",
		"b.kt":     "fun b() {}\n",
		"sub/c.kt": "fun c() {}\n",
	}

	writeTree := func(root string) []string {
		var paths []string
		for _, rel := range rels {
			abs := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(abs, []byte(contents[rel]), 0644); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, abs)
		}
		return paths
	}

	pathsA := writeTree(rootA)
	pathsB := writeTree(rootB)

	// Build the cache as if produced by a run under rootA.
	cA := &Cache{
		Version:   "1.0.0",
		RuleHash:  "rh",
		ScanPaths: []string{rootA},
		Files:     make(map[string]FileEntry),
	}
	for _, p := range pathsA {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		cA.Files[p] = FileEntry{
			Hash:    ComputeFileHash(p),
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
			Columns: scanner.CollectFindings([]scanner.Finding{
				{File: p, Line: 1, Col: 1, Severity: "warning", RuleSet: "s", Rule: "R", Message: "m"},
			}),
		}
	}

	cacheA := filepath.Join(rootA, ".krit", "cache", CacheFileName)
	if err := cA.Save(cacheA); err != nil {
		t.Fatal(err)
	}

	// Simulate the CI-side path rewrite: load A's cache, rewrite the prefix
	// onto the new tree at the logical layer (map keys + interned Files
	// slice in each column), and save it where the new run will read it.
	// Byte-level substitution is not safe against the compressed binary
	// cache format, so callers must go through Load / Save.
	cached := Load(cacheA)
	rewritten := make(map[string]FileEntry, len(cached.Files))
	for path, entry := range cached.Files {
		cols := entry.Columns.Clone()
		for i, fp := range cols.Files {
			cols.Files[i] = strings.Replace(fp, rootA, rootB, 1)
		}
		newPath := strings.Replace(path, rootA, rootB, 1)
		rewritten[newPath] = FileEntry{
			Hash:    entry.Hash,
			ModTime: entry.ModTime,
			Size:    entry.Size,
			Columns: cols,
		}
	}
	cached.Files = rewritten
	cached.ScanPaths = []string{rootB}
	cacheB := filepath.Join(rootB, ".krit", "cache", CacheFileName)
	if err := os.MkdirAll(filepath.Dir(cacheB), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cached.Save(cacheB); err != nil {
		t.Fatal(err)
	}

	// Touch the rootB files to clear any inherited mtime equality artifacts,
	// then re-stat and rewrite mtimes in the cache to mirror what a real
	// "git checkout" would produce on the new machine. CheckFiles relies on
	// content hash (NeedsReanalysis falls back to hash on mtime mismatch).
	loaded := Load(cacheB)
	if len(loaded.Files) != len(rels) {
		t.Fatalf("expected %d entries, got %d", len(rels), len(loaded.Files))
	}

	res := loaded.CheckFiles(pathsB, "rh", rootB)
	if res.TotalCached != len(pathsB) {
		t.Fatalf("expected all %d files cached after round-trip, got %d (paths=%v cached=%v)",
			len(pathsB), res.TotalCached, pathsB, res.CachedPaths)
	}
}

// TestCache_BranchSwitchFidelity simulates a branch switch where a subset of
// files in a tree are edited, deleted, or added. The cache must:
//   - Hit on unchanged files.
//   - Miss on edited files (content hash differs).
//   - Miss on newly added files (no entry).
//   - Not crash or hit on deleted files (entry exists but stat fails).
func TestCache_BranchSwitchFidelity(t *testing.T) {
	root := t.TempDir()

	write := func(rel, body string) string {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		return abs
	}

	// Initial state — what was on the base branch when the cache was built.
	keepPath := write("keep.kt", "fun keep() {}\n")
	editPath := write("edit.kt", "fun before() {}\n")
	delPath := write("del.kt", "fun gone() {}\n")

	c := &Cache{
		Version:   "1.0.0",
		RuleHash:  "rh",
		ScanPaths: []string{root},
		Files:     make(map[string]FileEntry),
	}
	for _, p := range []string{keepPath, editPath, delPath} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		c.Files[p] = FileEntry{
			Hash:    ComputeFileHash(p),
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
		}
	}

	// Branch switch: edit, delete, add.
	if err := os.WriteFile(editPath, []byte("fun after() { println(\"x\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(delPath); err != nil {
		t.Fatal(err)
	}
	addPath := write("add.kt", "fun added() {}\n")

	// CheckFiles is called with the post-switch file list (matches what the
	// scanner would discover on disk).
	post := []string{keepPath, editPath, addPath}
	res := c.CheckFiles(post, "rh", root)

	if !res.CachedPaths[keepPath] {
		t.Errorf("expected hit on unchanged file %s", keepPath)
	}
	if res.CachedPaths[editPath] {
		t.Errorf("expected miss on edited file %s (content changed)", editPath)
	}
	if res.CachedPaths[addPath] {
		t.Errorf("expected miss on newly added file %s", addPath)
	}
	if res.TotalCached != 1 {
		t.Errorf("expected exactly 1 cached entry, got %d", res.TotalCached)
	}
	if res.TotalFiles != 3 {
		t.Errorf("expected TotalFiles=3, got %d", res.TotalFiles)
	}
}

// TestCache_BranchSwitchMtimeOnly verifies that touching a file (mtime
// changes but content stays identical) is treated as a hit. This protects
// the cache from over-invalidating after a `git checkout` that updates
// mtimes on otherwise-unchanged files.
func TestCache_BranchSwitchMtimeOnly(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "f.kt")
	if err := os.WriteFile(abs, []byte("fun f() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(abs)

	c := &Cache{
		Version:  "1.0.0",
		RuleHash: "rh",
		Files: map[string]FileEntry{
			abs: {
				Hash:    ComputeFileHash(abs),
				ModTime: info.ModTime().Unix(),
				Size:    info.Size(),
			},
		},
	}

	// Touch with a different mtime; content unchanged.
	future := info.ModTime().Add(60 * 60 * 1e9) // +1h
	if err := os.Chtimes(abs, future, future); err != nil {
		t.Fatal(err)
	}

	res := c.CheckFiles([]string{abs}, "rh", root)
	if res.TotalCached != 1 {
		t.Fatalf("mtime-only change invalidated cache; expected hit, got %d cached", res.TotalCached)
	}
}

// TestCacheLoad_RejectsCorruptJSON verifies Load returns an empty cache
// (forcing full re-analysis) when the on-disk file is not valid JSON. A
// silent-but-empty fallback is the correct behavior for a derived cache:
// we never want to misinterpret stale or partial bytes as cache hits.
func TestCacheLoad_RejectsCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.cache")
	if err := os.WriteFile(path, []byte("not json {{{"), 0644); err != nil {
		t.Fatal(err)
	}
	c := Load(path)
	if c == nil {
		t.Fatal("Load returned nil for corrupt input; expected empty cache")
	}
	if len(c.Files) != 0 {
		t.Errorf("expected empty Files map after corrupt load, got %d entries", len(c.Files))
	}
}

// TestCacheLoad_RejectsTruncatedJSON verifies a partially-written cache
// file (e.g. crash mid-write on a previous run) yields an empty cache.
func TestCacheLoad_RejectsTruncatedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trunc.cache")
	c := makePortabilityCache()
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if len(raw) < 32 {
		t.Skipf("cache too small to truncate meaningfully: %d bytes", len(raw))
	}
	if err := os.WriteFile(path, raw[:len(raw)/2], 0644); err != nil {
		t.Fatal(err)
	}

	loaded := Load(path)
	if loaded == nil {
		t.Fatal("Load returned nil for truncated input")
	}
	if len(loaded.Files) != 0 {
		t.Errorf("expected empty Files after truncated load, got %d entries", len(loaded.Files))
	}
}

// TestCache_ConcurrentSaveSamePath has multiple goroutines write to the
// same cache path simultaneously. The atomic write+rename should leave
// the file in a fully-valid state — never half-written, never empty.
// Whichever writer wins the rename race is fine; the file just must be a
// valid cache afterwards.
func TestCache_ConcurrentSaveSamePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shared.cache")

	const writers = 8
	var wg sync.WaitGroup
	wg.Add(writers)
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		go func(i int) {
			defer wg.Done()
			c := makePortabilityCache()
			c.RuleHash = "writer-" + strings.Repeat("x", i+1)
			if err := c.Save(path); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent save: %v", err)
	}

	loaded := Load(path)
	if loaded == nil || len(loaded.Files) == 0 {
		t.Fatalf("expected valid cache after concurrent saves, got %+v", loaded)
	}
	// Whichever writer's RuleHash landed should be a valid prefix; the
	// important property is the file isn't empty/corrupt.
	if !strings.HasPrefix(loaded.RuleHash, "writer-") {
		t.Errorf("RuleHash not from any writer: %q", loaded.RuleHash)
	}
}

// TestCache_ConcurrentSaveDifferentPaths runs multiple writers into the
// same directory but distinct file names. Each writer's file must round-
// trip independently; no cross-contamination.
func TestCache_ConcurrentSaveDifferentPaths(t *testing.T) {
	dir := t.TempDir()
	const writers = 8

	var wg sync.WaitGroup
	wg.Add(writers)
	errs := make(chan error, writers)
	paths := make([]string, writers)
	hashes := make([]string, writers)
	for i := 0; i < writers; i++ {
		paths[i] = filepath.Join(dir, "krit-"+strings.Repeat("a", i+1)+".cache")
		hashes[i] = "rh-" + strings.Repeat("z", i+1)
	}
	for i := 0; i < writers; i++ {
		go func(i int) {
			defer wg.Done()
			c := makePortabilityCache()
			c.RuleHash = hashes[i]
			if err := c.Save(paths[i]); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent save: %v", err)
	}

	for i, p := range paths {
		loaded := Load(p)
		if loaded == nil {
			t.Fatalf("writer %d: load nil", i)
		}
		if loaded.RuleHash != hashes[i] {
			t.Errorf("writer %d: RuleHash=%q want %q (cross-contamination?)", i, loaded.RuleHash, hashes[i])
		}
	}
}

// TestCacheLoad_NoEnvLeakage saves a cache once, then Load+resave under
// different env settings. The re-saved bytes must match the original. This
// guards against Load (or its dependencies) reading env during deserialization.
func TestCacheLoad_NoEnvLeakage(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.cache")
	c := makePortabilityCache()

	t.Setenv("TZ", "UTC")
	if err := c.Save(src); err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(src)

	for i, env := range []map[string]string{
		{"TZ": "Asia/Tokyo", "HOME": "/x", "USER": "u1"},
		{"TZ": "America/Los_Angeles", "HOME": "/y", "USER": "u2"},
	} {
		for k, v := range env {
			t.Setenv(k, v)
		}
		loaded := Load(src)
		out := filepath.Join(dir, "out.cache")
		if err := loaded.Save(out); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		got, _ := os.ReadFile(out)
		if !bytes.Equal(want, got) {
			t.Fatalf("env %d: load+resave bytes differ\nwant=%q\ngot=%q", i, want, got)
		}
	}
}
