package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestComputeStaleOraclePaths_FiltersGeneratedWhenDisabled is the
// regression guard for the warm-rerun freshness-gate phantom-miss bug.
// Before the fix, the gate compared every .kt path against the bundle
// manifest — including files under /generated/ that the parse phase
// already filters out. Those paths weren't in the manifest (parse had
// dropped them before the manifest write), so the gate marked them as
// stale, the oracle layer treated them as ForcedMisses, and krit-fir
// launched a one-shot JVM to analyze ~98 files on every warm rerun
// against the kotlin repo. ~80s of pointless JVM startup cost per
// warm scan.
//
// The fix applies the same /generated/ filter to the gate's input
// that the parse phase will apply. With includeGenerated=false, paths
// under /generated/ never reach the manifest comparison.
//
// This test asserts the filter is applied symmetrically: we never
// load a manifest (no prior bundle) so the function short-circuits to
// nil for the non-generated cases — what we care about is that the
// generated paths are pruned from the input slice the gate considers.
// We exercise that via a strings.Contains check on the function's
// own filter helper, since the gate's full path requires a real
// repoDir + cache layout to load a manifest.
func TestFilterGeneratedPathStrings_DropsGeneratedKotlin(t *testing.T) {
	t.Helper()
	in := []string{
		"src/main/kotlin/Foo.kt",
		filepath.FromSlash("a/b/generated/_ArraysNative.kt"),
		filepath.FromSlash("kotlin-native/runtime/src/main/kotlin/generated/_CollectionsNative.kt"),
		"src/main/kotlin/Bar.kt",
	}
	out := filterGeneratedPathStrings(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 retained paths after /generated/ filter, got %d: %v", len(out), out)
	}
	for _, p := range out {
		if strings.Contains(filepath.ToSlash(p), "/generated/") {
			t.Errorf("filter retained generated path: %q", p)
		}
	}
}

func TestComputeStaleOraclePaths_IncludesDependentsAndRefreshesMerge(t *testing.T) {
	dir := t.TempDir()
	dependency := filepath.Join(dir, "Dependency.kt")
	caller := filepath.Join(dir, "Caller.kt")
	if err := os.WriteFile(dependency, []byte("package demo\nfun dependency() = \"value\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caller, []byte("package demo\nfun caller() = dependency().length\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cacheDir, err := oracle.CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	oldCallerFacts := &oracle.File{Expressions: map[string]*oracle.ExpressionType{
		"2:16": {Type: "kotlin.String", Nullable: false, StartByte: 28, EndByte: 40},
	}}
	oldData := &oracle.Data{Version: 1, Files: map[string]*oracle.File{
		dependency: {Package: "demo"},
		caller:     oldCallerFacts,
	}}
	deps := &oracle.CacheDepsFile{Version: 1, Files: map[string]*oracle.CacheDepsEntry{
		dependency: {DepPaths: nil},
		caller:     {DepPaths: []string{dependency}},
	}}
	if _, err := oracle.WriteFreshEntries(cacheDir, oldData, deps); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}
	typesPath := oracle.CachePath([]string{dir})
	if err := os.MkdirAll(filepath.Dir(typesPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(oldData)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(typesPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	manifestKey := scanner.FindingsBundleManifestKey(dir, []string{dir})
	manifest := scanner.FindingsBundleManifest{
		ContentHashes: map[string]string{},
		FileStats:     map[string]scanner.FileStat{},
	}
	for _, path := range []string{dependency, caller} {
		hash, err := oracle.ContentHash(path)
		if err != nil {
			t.Fatal(err)
		}
		manifest.ContentHashes[path] = hash
		stat, ok := scanner.StatFile(path)
		if !ok {
			t.Fatalf("StatFile(%s)", path)
		}
		manifest.FileStats[path] = stat
	}
	if err := scanner.SaveFindingsBundleManifest(dir, manifestKey, manifest); err != nil {
		t.Fatalf("SaveFindingsBundleManifest: %v", err)
	}

	if err := os.WriteFile(dependency, []byte("package demo\nfun dependency() = null\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := computeStaleOraclePaths([]string{dir}, []string{caller, dependency}, true, nil, false)
	if want := []string{caller, dependency}; !reflect.DeepEqual(stale, want) {
		t.Fatalf("StaleOraclePaths = %v, want changed plus reverse closure %v", stale, want)
	}

	freshCallerFacts := &oracle.File{Expressions: map[string]*oracle.ExpressionType{
		"2:16": {Type: "kotlin.String", Nullable: true, StartByte: 28, EndByte: 40},
	}}
	merged, _, err := oracle.MergeFreshIntoCachedTypes(typesPath, &oracle.Data{Version: 1, Files: map[string]*oracle.File{
		dependency: {Package: "demo"},
		caller:     freshCallerFacts,
	}})
	if err != nil {
		t.Fatalf("MergeFreshIntoCachedTypes: %v", err)
	}
	if got := merged.Files[caller].Expressions["2:16"].Nullable; !got {
		t.Fatal("fresh dependent expression fact did not win partial merge")
	}
}
