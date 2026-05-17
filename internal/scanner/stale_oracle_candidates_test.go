package scanner

import (
	"reflect"
	"sort"
	"testing"
)

func TestStaleOracleCandidates_NilStat_TreatsAllAsStale(t *testing.T) {
	t.Parallel()
	paths := []string{"/a.kt", "/b.kt"}
	stale := StaleOracleCandidates(paths, FindingsBundleManifest{}, nil)
	if !reflect.DeepEqual(stale, paths) {
		t.Errorf("stale = %v, want %v", stale, paths)
	}
}

func TestStaleOracleCandidates_NoPriorManifest_AllStale(t *testing.T) {
	t.Parallel()
	paths := []string{"/a.kt", "/b.kt"}
	stat := func(string) (FileStat, bool) {
		return FileStat{Size: 1, ModTimeUnixNano: 1}, true
	}
	stale := StaleOracleCandidates(paths, FindingsBundleManifest{}, stat)
	// With no manifest evidence, every path must be reported stale so
	// the caller doesn't reuse an oracle JSON we cannot verify.
	if !reflect.DeepEqual(stale, paths) {
		t.Errorf("stale = %v, want %v", stale, paths)
	}
}

func TestStaleOracleCandidates_AllMatching_Empty(t *testing.T) {
	t.Parallel()
	paths := []string{"/a.kt", "/b.kt"}
	prior := FindingsBundleManifest{
		FileStats: map[string]FileStat{
			"/a.kt": {Size: 10, ModTimeUnixNano: 100},
			"/b.kt": {Size: 20, ModTimeUnixNano: 200},
		},
		ContentHashes: map[string]string{"/a.kt": "x", "/b.kt": "y"},
	}
	stat := func(p string) (FileStat, bool) {
		return prior.FileStats[p], true
	}
	stale := StaleOracleCandidates(paths, prior, stat)
	if len(stale) != 0 {
		t.Errorf("stale = %v, want empty when stats match", stale)
	}
}

func TestStaleOracleCandidates_StatMismatch_FlaggedStale(t *testing.T) {
	t.Parallel()
	paths := []string{"/a.kt", "/b.kt"}
	prior := FindingsBundleManifest{
		FileStats: map[string]FileStat{
			"/a.kt": {Size: 10, ModTimeUnixNano: 100},
			"/b.kt": {Size: 20, ModTimeUnixNano: 200},
		},
	}
	// /b.kt's mtime moved forward — must be flagged stale; /a.kt unchanged.
	stat := func(p string) (FileStat, bool) {
		if p == "/b.kt" {
			return FileStat{Size: 20, ModTimeUnixNano: 999}, true
		}
		return prior.FileStats[p], true
	}
	stale := StaleOracleCandidates(paths, prior, stat)
	want := []string{"/b.kt"}
	if !reflect.DeepEqual(stale, want) {
		t.Errorf("stale = %v, want %v", stale, want)
	}
}

func TestStaleOracleCandidates_StatUnavailable_FlaggedStale(t *testing.T) {
	t.Parallel()
	paths := []string{"/a.kt", "/missing.kt"}
	prior := FindingsBundleManifest{
		FileStats: map[string]FileStat{
			"/a.kt": {Size: 10, ModTimeUnixNano: 100},
		},
	}
	// /missing.kt's stat returns !ok; that must mark it stale rather
	// than silently treating it as fresh.
	stat := func(p string) (FileStat, bool) {
		if p == "/missing.kt" {
			return FileStat{}, false
		}
		return prior.FileStats[p], true
	}
	stale := StaleOracleCandidates(paths, prior, stat)
	want := []string{"/missing.kt"}
	if !reflect.DeepEqual(stale, want) {
		t.Errorf("stale = %v, want %v", stale, want)
	}
}

func TestStaleOracleCandidates_PriorMissingPath_FlaggedStale(t *testing.T) {
	t.Parallel()
	// /b.kt has no entry in the prior manifest's FileStats — treat as
	// stale (it's effectively a new file from the cache's POV).
	paths := []string{"/a.kt", "/b.kt"}
	prior := FindingsBundleManifest{
		FileStats: map[string]FileStat{
			"/a.kt": {Size: 10, ModTimeUnixNano: 100},
		},
	}
	stat := func(p string) (FileStat, bool) {
		switch p {
		case "/a.kt":
			return prior.FileStats["/a.kt"], true
		case "/b.kt":
			return FileStat{Size: 5, ModTimeUnixNano: 500}, true
		}
		return FileStat{}, false
	}
	stale := StaleOracleCandidates(paths, prior, stat)
	want := []string{"/b.kt"}
	if !reflect.DeepEqual(stale, want) {
		t.Errorf("stale = %v, want %v", stale, want)
	}
}

func TestStaleOracleCandidates_LegacyManifestNoFileStats(t *testing.T) {
	t.Parallel()
	// Legacy manifests (pre-FileStats) only have ContentHashes. We
	// can't compare stats — paths present in ContentHashes are
	// trusted; new paths are flagged.
	paths := []string{"/a.kt", "/new.kt"}
	prior := FindingsBundleManifest{
		ContentHashes: map[string]string{"/a.kt": "hashA"},
	}
	stat := func(string) (FileStat, bool) {
		return FileStat{Size: 1, ModTimeUnixNano: 1}, true
	}
	stale := StaleOracleCandidates(paths, prior, stat)
	want := []string{"/new.kt"}
	if !reflect.DeepEqual(stale, want) {
		t.Errorf("stale = %v, want %v", stale, want)
	}
}

func TestStaleOracleCandidates_OrderPreserved(t *testing.T) {
	t.Parallel()
	// The result must reflect the input order so downstream
	// classifier behavior is stable across runs (helps debug perf
	// regressions).
	paths := []string{"/c.kt", "/a.kt", "/b.kt"}
	prior := FindingsBundleManifest{
		FileStats: map[string]FileStat{}, // every path is "new"
	}
	stat := func(string) (FileStat, bool) {
		return FileStat{Size: 1, ModTimeUnixNano: 1}, true
	}
	stale := StaleOracleCandidates(paths, prior, stat)
	sorted := append([]string(nil), stale...)
	sort.Strings(sorted)
	if reflect.DeepEqual(stale, sorted) {
		// If the result happens to be sorted, this test would still pass
		// for the wrong reason — make the assertion explicit on input order.
		if !reflect.DeepEqual(stale, paths) {
			t.Errorf("stale = %v, want input order %v", stale, paths)
		}
	}
}
