package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// A relative `krit .` oracle run through the one-shot JVM (no daemon) must
// analyze the same files as the absolute-root run: krit-types matches --files
// by exact equality with absolute paths, so an unabsolutized miss list made
// it analyze 0 files and cache them as jar-skipped markers that then poisoned
// absolute-root runs too. The sharded variant splits the misses across two
// JVMs, each of which is sent and re-spelled separately.
func TestOracleOneShotRelativeRootMatchesAbsoluteRoot(t *testing.T) {
	for _, shards := range []int{1, 2} {
		t.Run("shards="+strconv.Itoa(shards), func(t *testing.T) {
			jar, opts := oneShotRelativeSetup(t, shards)
			relDir := []string{filepath.Join("src", "main", "kotlin")}

			rel := runOneShotCached(t, jar, relDir, "", opts)
			if len(rel.Files) != 2 || rel.Files[relA] == nil || rel.Files[relB] == nil {
				t.Fatalf("relative root files = %v, want %q and %q", keys(rel.Files), relA, relB)
			}
			if !hasDiagnostic(rel.Files[relA], "UNNECESSARY_NOT_NULL_ASSERTION") {
				t.Fatalf("relative root lost the diagnostic: %+v", rel.Files[relA])
			}

			// The cache entries written by the relative run carry the
			// caller's spelling of their dependency edges, not absolute ones.
			cacheDir, err := oracle.CacheDir(".")
			if err != nil {
				t.Fatal(err)
			}
			hits, misses := oracle.ClassifyFilesScopedV3(cacheDir, []string{relA, relB}, "", "", opts.Backend.CacheApproximation())
			if len(hits) != 2 || len(misses) != 0 {
				t.Fatalf("cache: hits=%d misses=%v", len(hits), misses)
			}
			for _, hit := range hits {
				if hit.Crashed {
					t.Fatalf("cached %q as a crash: %s", hit.FilePath, hit.CrashError)
				}
				for _, dep := range hit.Closure.DepPaths {
					if filepath.IsAbs(dep) {
						t.Fatalf("cache entry for %q has absolute dep %q", hit.FilePath, dep)
					}
				}
			}

			// Same cache (repoDir "."), absolute roots: must not be served poison.
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			abs := runOneShotCached(t, jar, []string{filepath.Join(cwd, "src", "main", "kotlin")}, "", opts)
			if len(abs.Files) != 2 {
				t.Fatalf("absolute root files = %v, want 2", keys(abs.Files))
			}
			for _, name := range []string{relA, relB} {
				got := abs.Files[filepath.Join(cwd, name)]
				if got == nil || got.Package != rel.Files[name].Package {
					t.Fatalf("absolute root facts for %q differ from relative: %+v vs %+v", name, got, rel.Files[name])
				}
			}
			if !hasDiagnostic(abs.Files[filepath.Join(cwd, relA)], "UNNECESSARY_NOT_NULL_ASSERTION") {
				t.Fatalf("absolute root lost the diagnostic (poisoned cache?)")
			}
		})
	}
}

// A narrowed oracle filter (the absolute paths of the files the active rules
// need) must still select files on a relative scan root.
func TestOracleOneShotRelativeRootNarrowedFilter(t *testing.T) {
	jar, opts := oneShotRelativeSetup(t, 1)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(t.TempDir(), "files.txt")
	if err := os.WriteFile(list, []byte(filepath.Join(cwd, relA)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel := runOneShotCached(t, jar, []string{filepath.Join("src", "main", "kotlin")}, list, opts)
	if len(rel.Files) != 1 || rel.Files[relA] == nil {
		t.Fatalf("narrowed relative run files = %v, want only %q", keys(rel.Files), relA)
	}
	if !hasDiagnostic(rel.Files[relA], "UNNECESSARY_NOT_NULL_ASSERTION") {
		t.Fatalf("narrowed relative run lost the diagnostic: %+v", rel.Files[relA])
	}
}

func oneShotRelativeSetup(t *testing.T, shards int) (string, oracle.InvocationOptions) {
	t.Helper()
	var kaa relativeOracleCase
	for _, tc := range relativeOracleCases() {
		if tc.name == "kaa" {
			kaa = tc
		}
	}
	jar := relativeOracleProject(t, kaa)
	t.Setenv("KRIT_DAEMON_CACHE", "off")
	t.Setenv("KRIT_TYPES_SHARDS", strconv.Itoa(shards))
	return jar, oracle.InvocationOptions{Backend: kaa.backend}
}

func runOneShotCached(t *testing.T, jar string, sourceDirs []string, filterList string, opts oracle.InvocationOptions) *oracle.Data {
	t.Helper()
	out := filepath.Join(t.TempDir(), "types.json")
	if _, err := oracle.InvokeCachedWithOptions(jar, sourceDirs, ".", out, filterList, false, nil, opts); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var data oracle.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return &data
}
