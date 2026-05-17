package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeOracleData_FreshWinsOnFileOverlap(t *testing.T) {
	t.Parallel()
	cached := &Data{
		Version: 1,
		Files: map[string]*File{
			"/a.kt": {Package: "p", Declarations: []*Class{{FQN: "p.A"}}},
			"/b.kt": {Package: "p", Declarations: []*Class{{FQN: "p.B"}}},
		},
		Dependencies: map[string]*Class{
			"java.lang.String": {FQN: "java.lang.String", Kind: "class"},
		},
	}
	fresh := &Data{
		Files: map[string]*File{
			// /a.kt got a new public function — fresh entry replaces cached.
			"/a.kt": {Package: "p", Declarations: []*Class{{FQN: "p.A", Members: []*Member{{Name: "newFn"}}}}},
		},
	}
	merged := mergeOracleData(cached, fresh)
	// Both files survive; /a.kt reflects fresh content.
	if len(merged.Files) != 2 {
		t.Fatalf("merged.Files = %d, want 2", len(merged.Files))
	}
	gotA := merged.Files["/a.kt"]
	if gotA == nil || len(gotA.Declarations) != 1 || len(gotA.Declarations[0].Members) != 1 {
		t.Errorf("merged /a.kt = %+v, want fresh entry with newFn member", gotA)
	}
	if merged.Files["/b.kt"] == nil {
		t.Errorf("merged dropped /b.kt — cached file with no fresh overlap must survive")
	}
}

func TestMergeOracleData_AddingNewFile(t *testing.T) {
	t.Parallel()
	cached := &Data{Files: map[string]*File{"/a.kt": {Package: "p"}}}
	fresh := &Data{Files: map[string]*File{"/c.kt": {Package: "p"}}}
	merged := mergeOracleData(cached, fresh)
	if _, ok := merged.Files["/a.kt"]; !ok {
		t.Errorf("merged missing /a.kt")
	}
	if _, ok := merged.Files["/c.kt"]; !ok {
		t.Errorf("merged missing /c.kt — fresh entry for a brand-new file must land")
	}
}

func TestMergeOracleData_DependenciesUnion(t *testing.T) {
	t.Parallel()
	cached := &Data{Dependencies: map[string]*Class{
		"java.lang.String": {FQN: "java.lang.String"},
		"java.lang.Object": {FQN: "java.lang.Object"},
	}}
	fresh := &Data{Dependencies: map[string]*Class{
		"java.lang.String": {FQN: "java.lang.String", Kind: "class"}, // fresh wins
		"kotlin.Int":       {FQN: "kotlin.Int"},                      // new
	}}
	merged := mergeOracleData(cached, fresh)
	if len(merged.Dependencies) != 3 {
		t.Errorf("merged.Dependencies = %d, want 3 (java.lang.String overwritten, java.lang.Object preserved, kotlin.Int added)", len(merged.Dependencies))
	}
	if merged.Dependencies["java.lang.String"].Kind != "class" {
		t.Errorf("fresh dep did not win on FQN overlap")
	}
}

func TestMergeOracleData_VersionAndKotlinVersion(t *testing.T) {
	t.Parallel()
	cached := &Data{Version: 1, KotlinVersion: "1.9.0"}
	// Empty fresh — partial-reanalyze response often omits top-level
	// fields because they describe the whole project, not the subset.
	fresh := &Data{}
	merged := mergeOracleData(cached, fresh)
	if merged.Version != 1 {
		t.Errorf("merged.Version = %d, want 1 (preserve cached when fresh.Version == 0)", merged.Version)
	}
	if merged.KotlinVersion != "1.9.0" {
		t.Errorf("merged.KotlinVersion = %q, want %q", merged.KotlinVersion, "1.9.0")
	}
	// Fresh that DOES carry these fields should override.
	fresh = &Data{Version: 2, KotlinVersion: "2.0.0"}
	merged = mergeOracleData(cached, fresh)
	if merged.Version != 2 || merged.KotlinVersion != "2.0.0" {
		t.Errorf("fresh non-zero fields did not override: %+v", merged)
	}
}

func TestMergeOracleData_NilInputs(t *testing.T) {
	t.Parallel()
	// Defensive: neither nil should panic; an absent map becomes empty.
	merged := mergeOracleData(nil, nil)
	if merged == nil || merged.Files == nil || merged.Dependencies == nil {
		t.Errorf("nil/nil merge produced unusable result: %+v", merged)
	}
}

func TestMergeFreshIntoCachedTypes_RoundTripWritesMerged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	// Seed an on-disk cached types.json.
	cached := &Data{
		Version: 1,
		Files: map[string]*File{
			"/a.kt": {Package: "p", Declarations: []*Class{{FQN: "p.A"}}},
		},
		Dependencies: map[string]*Class{},
	}
	raw, _ := json.Marshal(cached)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("seed types.json: %v", err)
	}
	fresh := &Data{Files: map[string]*File{
		"/a.kt": {Package: "p", Declarations: []*Class{{FQN: "p.A", Members: []*Member{{Name: "extra"}}}}},
		"/b.kt": {Package: "p"},
	}}
	merged, err := MergeFreshIntoCachedTypes(path, fresh)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if len(merged.Files) != 2 {
		t.Errorf("merged.Files = %d, want 2", len(merged.Files))
	}
	// Verify disk reflects merge so the next warm run sees the new content.
	roundTrip, err := readOracleJSON(path)
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}
	if len(roundTrip.Files) != 2 {
		t.Errorf("on-disk merged.Files = %d, want 2", len(roundTrip.Files))
	}
	if roundTrip.Files["/a.kt"] == nil || len(roundTrip.Files["/a.kt"].Declarations[0].Members) != 1 {
		t.Errorf("on-disk /a.kt did not reflect fresh content")
	}
}

func TestMergeFreshIntoCachedTypes_EmptyOutputPath(t *testing.T) {
	t.Parallel()
	// Defensive: empty path must not crash. Returns error so caller
	// falls back to analyzeAll instead of silently writing nothing.
	if _, err := MergeFreshIntoCachedTypes("", &Data{}); err == nil {
		t.Errorf("empty outputPath: want error, got nil")
	}
}

func TestMergeFreshIntoCachedTypes_SkipsWriteWhenFreshIsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	cached := &Data{Files: map[string]*File{"/a.kt": {Package: "p"}}, Dependencies: map[string]*Class{}}
	raw, _ := json.Marshal(cached)
	_ = os.WriteFile(path, raw, 0o644)
	priorStat, _ := os.Stat(path)
	// Empty fresh — the no-op merge must not re-marshal + rewrite the
	// 1MB JSON. Modtime stays unchanged.
	if _, err := MergeFreshIntoCachedTypes(path, &Data{Files: map[string]*File{}, Dependencies: map[string]*Class{}}); err != nil {
		t.Fatalf("merge: %v", err)
	}
	afterStat, _ := os.Stat(path)
	if !afterStat.ModTime().Equal(priorStat.ModTime()) {
		t.Errorf("empty-fresh merge wrote to disk (modtime changed): %v -> %v", priorStat.ModTime(), afterStat.ModTime())
	}
}

func TestMergeFreshIntoCachedTypes_NilFreshTreatedAsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	cached := &Data{Files: map[string]*File{"/a.kt": {Package: "p"}}, Dependencies: map[string]*Class{}}
	raw, _ := json.Marshal(cached)
	_ = os.WriteFile(path, raw, 0o644)
	// nil fresh means "no new facts" — the merge should pass through
	// cached content unchanged.
	merged, err := MergeFreshIntoCachedTypes(path, nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if len(merged.Files) != 1 || merged.Files["/a.kt"] == nil {
		t.Errorf("nil-fresh merge dropped cached content: %+v", merged.Files)
	}
}
