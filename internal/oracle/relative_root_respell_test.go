package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

// writeAbsoluteJVMResult plays the JVM: facts and cache deps keyed by the
// absolute paths it was sent. Every file depends on dep (also absolute).
func writeAbsoluteJVMResult(t *testing.T, freshPath, depsPath string, files []string, dep string) {
	t.Helper()
	data := &Data{Version: 1, Files: map[string]*File{}}
	deps := &CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{}}
	for _, f := range files {
		data.Files[f] = &File{Package: "p"}
		deps.Files[f] = &CacheDepsEntry{DepPaths: []string{dep}, PerFileDeps: map[string]*Class{}}
	}
	for path, v := range map[string]any{freshPath: data, depsPath: deps} {
		body, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func assertRelativeResult(t *testing.T, fresh *Data, deps *CacheDepsFile, wantFiles []string, wantDep string) {
	t.Helper()
	if fresh == nil || len(fresh.Files) != len(wantFiles) {
		t.Fatalf("fresh files = %v, want %v", dataKeysOf(fresh), wantFiles)
	}
	for _, f := range wantFiles {
		if fresh.Files[f] == nil {
			t.Fatalf("fresh keys not re-spelled: %v, want %v", dataKeysOf(fresh), wantFiles)
		}
		entry := deps.Files[f]
		if entry == nil {
			t.Fatalf("cache deps not re-keyed: %v, want %q", deps.Files, f)
		}
		if len(entry.DepPaths) != 1 || entry.DepPaths[0] != wantDep {
			t.Fatalf("dep paths of %q = %v, want [%q]", f, entry.DepPaths, wantDep)
		}
	}
}

func dataKeysOf(d *Data) []string {
	var out []string
	if d != nil {
		for k := range d.Files {
			out = append(out, k)
		}
	}
	return out
}

// The unsharded one-shot path (runMissAnalysis' fallback) reads the JVM's
// absolute-keyed result through readOneShotMissResult.
func TestReadOneShotMissResult_RespellsFactsAndDeps(t *testing.T) {
	dir := chdirProject(t, map[string]string{"src/A.kt": "class A\n", "src/B.kt": "class B\n"})
	a, b := filepath.Join("src", "A.kt"), filepath.Join("src", "B.kt")
	fresh, depsPath := filepath.Join(t.TempDir(), "f.json"), filepath.Join(t.TempDir(), "d.json")
	writeAbsoluteJVMResult(t, fresh, depsPath, []string{filepath.Join(dir, a), filepath.Join(dir, b)}, filepath.Join(dir, "src", "Dep.kt"))

	data, deps, depsErr, err := readOneShotMissResult(perf.New(false), fresh, depsPath, []string{"src"}, []string{a, b})
	if err != nil || depsErr != nil {
		t.Fatalf("err=%v depsErr=%v", err, depsErr)
	}
	assertRelativeResult(t, data, deps, []string{a, b}, filepath.Join("src", "Dep.kt"))
}

// The sharded one-shot path re-spells each shard's result before merging.
func TestShardedOneShot_RespellsFactsAndDeps(t *testing.T) {
	dir := chdirProject(t, map[string]string{"src/A.kt": "class A\n", "src/B.kt": "class B\n"})
	a, b := filepath.Join("src", "A.kt"), filepath.Join("src", "B.kt")
	runner := func(_ string, _ []string, listPath, freshPath, depsPath string, _ bool, _ perf.Tracker) error {
		list, err := os.ReadFile(listPath)
		if err != nil {
			return err
		}
		var files []string
		for _, line := range strings.Fields(string(list)) {
			if !filepath.IsAbs(line) {
				t.Errorf("shard --files entry %q is not absolute", line)
			}
			files = append(files, line)
		}
		writeAbsoluteJVMResult(t, freshPath, depsPath, files, filepath.Join(dir, "src", "Dep.kt"))
		return nil
	}
	fresh, deps, err := runKritTypesCachedShardedWithRunner("jar", []string{"src"}, []string{a, b}, 2, false, perf.New(false), runner)
	if err != nil {
		t.Fatal(err)
	}
	assertRelativeResult(t, fresh, deps, []string{a, b}, filepath.Join("src", "Dep.kt"))
}

// The oracle filter list holds absolute paths; a relative scan root's files
// must still intersect with it.
func TestApplyOracleFilter_RelativeFilesMatchAbsoluteList(t *testing.T) {
	dir := chdirProject(t, map[string]string{"src/A.kt": "class A\n", "src/B.kt": "class B\n"})
	list := filepath.Join(t.TempDir(), "files.txt")
	if err := os.WriteFile(list, []byte(filepath.Join(dir, "src", "B.kt")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := applyOracleFilter([]string{filepath.Join("src", "A.kt"), filepath.Join("src", "B.kt")}, list, perf.New(false), false)
	if len(got) != 1 || got[0] != filepath.Join("src", "B.kt") {
		t.Fatalf("filtered = %v, want the caller's spelling of B.kt only", got)
	}
}

// A lone oversized file the JVM legitimately skips keeps its
// marker, written under the absolute path so both a relative and an absolute
// root hit it on the next run, and no warning is emitted.
func TestWriteSkippedPoisonEntries_LoneSkippedFileMarkedAndHitFromBothRoots(t *testing.T) {
	dir := chdirProject(t, map[string]string{"src/Big.kt": "class Big\n"})
	if err := os.Truncate(filepath.Join(dir, "src", "Big.kt"), maxJVMSourceBytes+1); err != nil {
		t.Fatal(err)
	}
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	warnings := captureWarnings(t)
	rel := filepath.Join("src", "Big.kt")

	writeSkippedPoisonEntries(nil, cacheDir, []string{"src"}, []string{rel}, &Data{Files: map[string]*File{}}, &CacheDepsFile{}, "", "", perf.New(false), false)

	if warnings.Len() != 0 {
		t.Fatalf("unexpected warning: %q", warnings.String())
	}
	for _, p := range []string{rel, filepath.Join(dir, rel)} {
		hits, misses := ClassifyFiles(cacheDir, []string{p})
		if len(hits) != 1 || len(misses) != 0 || !hits[0].Crashed {
			t.Fatalf("%q: hits=%d misses=%v, want the skipped marker as a hit", p, len(hits), misses)
		}
	}
}

// A types.json is keyed by the spelling of the scan that produced it; a run
// with a different spelling must not reuse it.
func TestTypesJSONSatisfiesForPaths_KeyBase(t *testing.T) {
	dir := chdirProject(t, nil)
	path := filepath.Join(dir, "types.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scope := StoreScope{Backend: BackendFIR}
	rel, abs := []string{"."}, []string{dir}
	if err := RecordTypesFactsForPaths(path, false, rel, scope); err != nil {
		t.Fatal(err)
	}
	build := recordedBuildToken(t, path)
	if !TypesJSONSatisfiesForPaths(path, false, scope, build, rel) {
		t.Error("a relative-root run should reuse its own types.json")
	}
	if TypesJSONSatisfiesForPaths(path, false, scope, build, abs) {
		t.Error("an absolute-root run reused a types.json keyed by relative paths")
	}
	if err := RecordTypesFactsForPaths(path, false, abs, scope); err != nil {
		t.Fatal(err)
	}
	if TypesJSONSatisfiesForPaths(path, false, scope, build, rel) {
		t.Error("a relative-root run reused a types.json recorded for absolute roots")
	}
	if !TypesJSONSatisfiesForPaths(path, false, scope, build, abs) {
		t.Error("an absolute-root run should reuse its own types.json")
	}
}

func recordedBuildToken(t *testing.T, typesPath string) string {
	t.Helper()
	rec, ok := readTypesFacts(typesPath)
	if !ok {
		t.Fatal("no facts record")
	}
	return rec.BuildToken
}
