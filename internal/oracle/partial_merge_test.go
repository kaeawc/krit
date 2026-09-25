package oracle

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func statPresent(string) error { return nil }

func TestMergeOracleData_FreshWinsOnFileOverlap(t *testing.T) {
	cached := &Data{
		Version: 1,
		Files: map[string]*File{
			"/a.kt": {Package: "p", Declarations: []*Class{{FQN: "p.A"}}},
			"/b.kt": {Package: "p", Declarations: []*Class{{FQN: "p.B"}}},
		},
		Dependencies: map[string]*Class{"java.lang.String": {FQN: "java.lang.String", Kind: "class"}},
	}
	fresh := &Data{Files: map[string]*File{
		"/a.kt": {Package: "p", Declarations: []*Class{{FQN: "p.A", Members: []*Member{{Name: "newFn"}}}}},
	}}
	merged, pruned := mergeOracleData(cached, fresh, statPresent)
	if pruned {
		t.Fatal("merge reported pruning for existing cached files")
	}
	if len(merged.Files) != 2 || len(merged.Files["/a.kt"].Declarations[0].Members) != 1 || merged.Files["/b.kt"] == nil {
		t.Fatalf("unexpected merged files: %+v", merged.Files)
	}
}

func TestMergeOracleData_AddingNewFile(t *testing.T) {
	cached := &Data{Files: map[string]*File{"/a.kt": {Package: "p"}}}
	fresh := &Data{Files: map[string]*File{"/c.kt": {Package: "p"}}}
	merged, pruned := mergeOracleData(cached, fresh, statPresent)
	if pruned || merged.Files["/a.kt"] == nil || merged.Files["/c.kt"] == nil {
		t.Fatalf("merged = %+v, pruned = %v", merged, pruned)
	}
}

func TestMergeOracleData_DependenciesUnion(t *testing.T) {
	cached := &Data{Dependencies: map[string]*Class{
		"java.lang.String": {FQN: "java.lang.String"}, "java.lang.Object": {FQN: "java.lang.Object"},
	}}
	fresh := &Data{Dependencies: map[string]*Class{
		"java.lang.String": {FQN: "java.lang.String", Kind: "class"}, "kotlin.Int": {FQN: "kotlin.Int"},
	}}
	merged, pruned := mergeOracleData(cached, fresh, statPresent)
	if pruned || len(merged.Dependencies) != 3 || merged.Dependencies["java.lang.String"].Kind != "class" {
		t.Fatalf("unexpected dependency merge: %+v (pruned %v)", merged.Dependencies, pruned)
	}
}

func TestMergeOracleData_VersionAndKotlinVersion(t *testing.T) {
	cached := &Data{Version: 1, KotlinVersion: "1.9.0"}
	merged, pruned := mergeOracleData(cached, &Data{}, statPresent)
	if pruned || merged.Version != 1 || merged.KotlinVersion != "1.9.0" {
		t.Fatalf("empty fresh changed cached versions: %+v (pruned %v)", merged, pruned)
	}
	merged, pruned = mergeOracleData(cached, &Data{Version: 2, KotlinVersion: "2.0.0"}, statPresent)
	if pruned || merged.Version != 2 || merged.KotlinVersion != "2.0.0" {
		t.Fatalf("fresh versions did not override cache: %+v (pruned %v)", merged, pruned)
	}
}

func TestMergeOracleData_NilInputs(t *testing.T) {
	merged, pruned := mergeOracleData(nil, nil, statPresent)
	if pruned || merged == nil || merged.Files == nil || merged.Dependencies == nil {
		t.Fatalf("nil/nil merge produced unusable result: %+v (pruned %v)", merged, pruned)
	}
}

func TestMergeOracleData_PrunesOnlyNotExistStatErrors(t *testing.T) {
	cached := &Data{Files: map[string]*File{"permission.kt": {}, "gone.kt": {}}}
	statErrs := map[string]error{
		"permission.kt": errors.New("permission denied"),
		"gone.kt":       fs.ErrNotExist,
	}
	merged, pruned := mergeOracleData(cached, &Data{}, func(path string) error { return statErrs[path] })
	if !pruned {
		t.Fatal("merge did not report pruning for missing cached file")
	}
	if merged.Files["permission.kt"] == nil {
		t.Fatal("merge dropped cached entry after a non-IsNotExist stat error")
	}
	if merged.Files["gone.kt"] != nil {
		t.Fatal("merge retained cached entry after IsNotExist")
	}
}

func TestKeepOnStatErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		keep bool
	}{
		{name: "not exist", err: fs.ErrNotExist, keep: false},
		{name: "wrapped not exist", err: &fs.PathError{Op: "stat", Path: "gone.kt", Err: fs.ErrNotExist}, keep: false},
		{name: "other error", err: errors.New("permission denied"), keep: true},
		{name: "nil", keep: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keepOnStatErr(tt.err); got != tt.keep {
				t.Fatalf("keepOnStatErr(%v) = %v, want %v", tt.err, got, tt.keep)
			}
		})
	}
}

func TestMergeFreshIntoCachedTypes_RoundTripWritesMerged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	aPath, bPath := filepath.Join(dir, "a.kt"), filepath.Join(dir, "b.kt")
	for _, sourcePath := range []string{aPath, bPath} {
		if err := os.WriteFile(sourcePath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cached := &Data{Version: 1, Files: map[string]*File{aPath: {Package: "p", Declarations: []*Class{{FQN: "p.A"}}}}, Dependencies: map[string]*Class{}}
	writeTypesJSON(t, path, cached)
	fresh := &Data{Files: map[string]*File{
		aPath: {Package: "p", Declarations: []*Class{{FQN: "p.A", Members: []*Member{{Name: "extra"}}}}},
		bPath: {Package: "p"},
	}}
	merged, pruned, err := MergeFreshIntoCachedTypes(path, fresh)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if pruned || len(merged.Files) != 2 {
		t.Fatalf("merged %d files, pruned %v; want 2 and false", len(merged.Files), pruned)
	}
	roundTrip, err := readOracleJSON(path)
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}
	if len(roundTrip.Files) != 2 || len(roundTrip.Files[aPath].Declarations[0].Members) != 1 {
		t.Fatalf("on-disk merge did not contain fresh facts: %+v", roundTrip.Files)
	}
}

func TestMergeFreshIntoCachedTypes_PrunesDeletedFileAndItsUnsafeDependencySignalsFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	gone := filepath.Join(dir, "Gone.kt")
	other := filepath.Join(dir, "Other.kt")
	for _, sourcePath := range []string{gone, other} {
		if err := os.WriteFile(sourcePath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cached := &Data{
		Files:        map[string]*File{gone: {Declarations: []*Class{{FQN: "p.Gone"}}}},
		Dependencies: map[string]*Class{"p.Gone": {FQN: "p.Gone", Kind: "class"}},
	}
	writeTypesJSON(t, path, cached)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	merged, pruned, err := MergeFreshIntoCachedTypes(path, &Data{Files: map[string]*File{other: {}}})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !pruned {
		t.Fatal("merge did not report pruning deleted Gone.kt")
	}
	if merged.Files[gone] != nil {
		t.Fatalf("merge retained deleted file entry %q", gone)
	}
	if merged.Dependencies["p.Gone"] == nil {
		t.Fatal("test setup expected union to retain p.Gone and require full-analysis fallback")
	}
}

func TestMergeFreshIntoCachedTypes_KeepsExistingUnscannedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	existing := filepath.Join(dir, "Unscanned.kt")
	if err := os.WriteFile(existing, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTypesJSON(t, path, &Data{Files: map[string]*File{existing: {Package: "p"}}})

	// This guards subset scans and --diff runs: a real source file that was
	// not scanned this invocation remains a valid oracle fact.
	merged, pruned, err := MergeFreshIntoCachedTypes(path, &Data{})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if pruned || merged.Files[existing] == nil {
		t.Fatalf("existing unscanned file was not kept: files=%v pruned=%v", merged.Files, pruned)
	}
}

func TestMergeFreshIntoCachedTypes_EmptyOutputPath(t *testing.T) {
	if _, _, err := MergeFreshIntoCachedTypes("", &Data{}); err == nil {
		t.Error("empty outputPath: want error, got nil")
	}
}

func TestMergeFreshIntoCachedTypes_SkipsWriteWhenFreshIsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	aPath := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(aPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTypesJSON(t, path, &Data{Files: map[string]*File{aPath: {Package: "p"}}, Dependencies: map[string]*Class{}})
	priorStat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := MergeFreshIntoCachedTypes(path, &Data{Files: map[string]*File{}, Dependencies: map[string]*Class{}}); err != nil {
		t.Fatalf("merge: %v", err)
	}
	afterStat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !afterStat.ModTime().Equal(priorStat.ModTime()) {
		t.Errorf("empty-fresh merge wrote to disk (modtime changed): %v -> %v", priorStat.ModTime(), afterStat.ModTime())
	}
}

func TestMergeFreshIntoCachedTypes_WritesWhenPruningWithEmptyFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	kept, gone := filepath.Join(dir, "kept.kt"), filepath.Join(dir, "gone.kt")
	for _, sourcePath := range []string{kept, gone} {
		if err := os.WriteFile(sourcePath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTypesJSON(t, path, &Data{Files: map[string]*File{kept: {}, gone: {}}, Dependencies: map[string]*Class{}})
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	merged, pruned, err := MergeFreshIntoCachedTypes(path, &Data{})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !pruned || merged.Files[gone] != nil {
		t.Fatalf("merge = (pruned %v, files %v), want deleted file pruned", pruned, merged.Files)
	}
	onDisk, err := readOracleJSON(path)
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}
	if onDisk.Files[gone] != nil {
		t.Fatal("pruned deleted file remained on disk")
	}
}

func TestMergeFreshIntoCachedTypes_NilFreshTreatedAsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	aPath := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(aPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTypesJSON(t, path, &Data{Files: map[string]*File{aPath: {Package: "p"}}, Dependencies: map[string]*Class{}})
	merged, pruned, err := MergeFreshIntoCachedTypes(path, nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if pruned || len(merged.Files) != 1 || merged.Files[aPath] == nil {
		t.Fatalf("nil-fresh merge changed cached content: %+v (pruned %v)", merged.Files, pruned)
	}
}

func writeTypesJSON(t *testing.T, path string, data *Data) {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("seed types.json: %v", err)
	}
}
