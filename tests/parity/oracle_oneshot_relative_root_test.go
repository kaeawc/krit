package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// A relative `krit .` oracle run through the one-shot JVM (no daemon) must
// analyze the same files as the absolute-root run: krit-types matches --files
// by exact equality with absolute paths, so an unabsolutized miss list made
// it analyze 0 files and cache them as jar-skipped markers that then poisoned
// absolute-root runs too.
func TestOracleOneShotRelativeRootMatchesAbsoluteRoot(t *testing.T) {
	var kaa relativeOracleCase
	for _, tc := range relativeOracleCases() {
		if tc.name == "kaa" {
			kaa = tc
		}
	}
	jar := relativeOracleProject(t, kaa)
	t.Setenv("KRIT_DAEMON_CACHE", "off")
	t.Setenv("KRIT_TYPES_SHARDS", "1")
	opts := oracle.InvocationOptions{Backend: kaa.backend}

	run := func(sourceDirs []string) *oracle.Data {
		t.Helper()
		out := filepath.Join(t.TempDir(), "types.json")
		if _, err := oracle.InvokeCachedWithOptions(jar, sourceDirs, ".", out, "", false, nil, opts); err != nil {
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

	rel := run([]string{filepath.Join("src", "main", "kotlin")})
	if len(rel.Files) != 2 || rel.Files[relA] == nil || rel.Files[relB] == nil {
		t.Fatalf("relative root files = %v, want %q and %q", keys(rel.Files), relA, relB)
	}
	if !hasDiagnostic(rel.Files[relA], "UNNECESSARY_NOT_NULL_ASSERTION") {
		t.Fatalf("relative root lost the diagnostic: %+v", rel.Files[relA])
	}

	// Same cache (repoDir "."), absolute roots: must not be served poison.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	abs := run([]string{filepath.Join(cwd, "src", "main", "kotlin")})
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
}
