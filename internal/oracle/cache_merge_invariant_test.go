package oracle

import (
	"strings"
	"testing"
)

// TestMergeData_PanicsOnDuplicateFilesKey enforces the documented
// disjointness invariant for shard merging. Today's caller
// (splitMissesForKAA) produces non-overlapping shards, so this panic
// is unreachable in production. Locking the contract surfaces a
// future regression at the producer rather than letting silent
// non-determinism propagate (#33).
func TestMergeData_PanicsOnDuplicateFilesKey(t *testing.T) {
	a := &Data{
		Version:      1,
		Files:        map[string]*File{"/a.kt": {Package: "p"}},
		Dependencies: map[string]*Class{},
	}
	b := &Data{
		Version:      1,
		Files:        map[string]*File{"/a.kt": {Package: "q"}}, // duplicate
		Dependencies: map[string]*Class{},
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate Files key")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(msg, "non-disjoint Files key") {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()
	_ = mergeData(a, b)
}

// TestMergeData_DisjointShardsMergeCleanly is the happy path: today's
// production callers behave this way and must not regress.
func TestMergeData_DisjointShardsMergeCleanly(t *testing.T) {
	a := &Data{
		Version:       1,
		KotlinVersion: "1.9.20",
		Files:         map[string]*File{"/a.kt": {Package: "p"}},
		Dependencies:  map[string]*Class{"p.A": {FQN: "p.A"}},
	}
	b := &Data{
		Version:      1,
		Files:        map[string]*File{"/b.kt": {Package: "q"}},
		Dependencies: map[string]*Class{"q.B": {FQN: "q.B"}},
	}
	got := mergeData(a, b)
	if len(got.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got.Files))
	}
	if got.Files["/a.kt"] == nil || got.Files["/b.kt"] == nil {
		t.Fatalf("missing file entries: %#v", got.Files)
	}
	if got.KotlinVersion != "1.9.20" {
		t.Fatalf("expected KotlinVersion=1.9.20, got %q", got.KotlinVersion)
	}
}

// TestMergeData_DependenciesAreLastWriteWins documents the explicit
// non-disjoint behavior for Dependencies (jar-resolved class facts):
// they ARE allowed to overlap across shards by design, with
// last-write-wins semantics.
func TestMergeData_DependenciesAreLastWriteWins(t *testing.T) {
	first := &Class{FQN: "p.X", Visibility: "public"}
	second := &Class{FQN: "p.X", Visibility: "internal"}
	a := &Data{
		Version: 1, Files: map[string]*File{},
		Dependencies: map[string]*Class{"p.X": first},
	}
	b := &Data{
		Version: 1, Files: map[string]*File{},
		Dependencies: map[string]*Class{"p.X": second},
	}
	got := mergeData(a, b)
	if got.Dependencies["p.X"].Visibility != "internal" {
		t.Fatalf("expected last-write-wins, got %#v", got.Dependencies["p.X"])
	}
}

// TestMergeCacheDeps_PanicsOnDuplicateFilesKey mirrors mergeData's
// invariant for the CacheDepsFile path.
func TestMergeCacheDeps_PanicsOnDuplicateFilesKey(t *testing.T) {
	a := &CacheDepsFile{
		Version: 1,
		Files:   map[string]*CacheDepsEntry{"/a.kt": {DepPaths: []string{"x"}}},
		Crashed: map[string]string{},
	}
	b := &CacheDepsFile{
		Version: 1,
		Files:   map[string]*CacheDepsEntry{"/a.kt": {DepPaths: []string{"y"}}},
		Crashed: map[string]string{},
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate Files key")
		}
		if !strings.Contains(r.(string), "non-disjoint Files key") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_ = mergeCacheDeps(a, b)
}

// TestMergeCacheDeps_PanicsOnDuplicateCrashedKey covers the parallel
// invariant for the Crashed map.
func TestMergeCacheDeps_PanicsOnDuplicateCrashedKey(t *testing.T) {
	a := &CacheDepsFile{
		Version: 1,
		Files:   map[string]*CacheDepsEntry{},
		Crashed: map[string]string{"/x.kt": "boom"},
	}
	b := &CacheDepsFile{
		Version: 1,
		Files:   map[string]*CacheDepsEntry{},
		Crashed: map[string]string{"/x.kt": "blast"},
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate Crashed key")
		}
		if !strings.Contains(r.(string), "non-disjoint Crashed key") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_ = mergeCacheDeps(a, b)
}

// TestMergeCacheDeps_DisjointShardsMergeCleanly confirms the
// production happy path is unaffected.
func TestMergeCacheDeps_DisjointShardsMergeCleanly(t *testing.T) {
	a := &CacheDepsFile{
		Version: 1, Approximation: "exact",
		Files:   map[string]*CacheDepsEntry{"/a.kt": {DepPaths: []string{"x"}}},
		Crashed: map[string]string{"/c.kt": "x"},
	}
	b := &CacheDepsFile{
		Version: 1,
		Files:   map[string]*CacheDepsEntry{"/b.kt": {DepPaths: []string{"y"}}},
		Crashed: map[string]string{"/d.kt": "y"},
	}
	got := mergeCacheDeps(a, b)
	if len(got.Files) != 2 || len(got.Crashed) != 2 {
		t.Fatalf("missing entries: %+v", got)
	}
	if got.Approximation != "exact" {
		t.Fatalf("approximation lost: %q", got.Approximation)
	}
}

// TestMergeData_NoParts returns a non-nil empty Data, preserving the
// caller contract observed by daemon_pool.go.
func TestMergeData_NoParts(t *testing.T) {
	got := mergeData()
	if got == nil {
		t.Fatal("expected non-nil empty Data")
	}
	if len(got.Files) != 0 || len(got.Dependencies) != 0 {
		t.Fatalf("expected empty maps, got %+v", got)
	}
}
