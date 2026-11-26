package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A convention root found from `.` and the model's absolute spelling of it
// are one root, so the oracle compiles (and sends krit-fir) it once (#1659).
func TestUnionSourceDirsDedupesSpellingsOfOneRoot(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join("app", "src", "main", "kotlin")
	lib := filepath.Join("lib", "src", "main", "kotlin")
	for _, dir := range []string{app, lib} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	absRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	model := []string{filepath.Join(absRoot, lib), filepath.Join(absRoot, app) + string(filepath.Separator), "./" + app}
	got := UnionSourceDirs(FindSourceDirs([]string{"."}), model)
	if want := []string{app, lib}; !slices.Equal(got, want) {
		t.Fatalf("UnionSourceDirs = %v, want %v (first spelling of each root)", got, want)
	}
}

// SamePaths ignores spelling only: krit-fir compares source roots and classpaths in order,
// repeats included, before it shares a compile.
func TestSamePathsIgnoresSpellingOnly(t *testing.T) {
	t.Chdir(t.TempDir())
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	a := []string{"app/src/main/kotlin", "lib/src/main/kotlin"}
	if !SamePaths(a, []string{filepath.Join(wd, "app/src/main/kotlin"), "./lib/src/main/kotlin/"}) {
		t.Fatal("another spelling of the same paths compared unequal")
	}
	for name, b := range map[string][]string{
		"reordered": {"lib/src/main/kotlin", "app/src/main/kotlin"},
		"repeated":  {"app/src/main/kotlin", "lib/src/main/kotlin", "app/src/main/kotlin"},
		"missing":   {"app/src/main/kotlin"},
	} {
		if SamePaths(a, b) {
			t.Fatalf("%s paths compared equal", name)
		}
	}
}
