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

func TestSameSourceDirSetIgnoresSpellingOrderAndDuplicates(t *testing.T) {
	t.Chdir(t.TempDir())
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	a := []string{"app/src/main/kotlin", "lib/src/main/kotlin", filepath.Join(wd, "app/src/main/kotlin")}
	b := []string{filepath.Join(wd, "lib/src/main/kotlin"), "./app/src/main/kotlin/"}
	if !SameSourceDirSet(a, b) {
		t.Fatalf("SameSourceDirSet(%v, %v) = false", a, b)
	}
	if SameSourceDirSet(a, b[:1]) {
		t.Fatal("a missing root compared equal")
	}
}

func TestSameClasspathKeepsOrder(t *testing.T) {
	t.Chdir(t.TempDir())
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	a := []string{"libs/a.jar", "libs/b.jar", filepath.Join(wd, "libs/a.jar")}
	if !SameClasspath(a, []string{filepath.Join(wd, "libs/a.jar"), "./libs/b.jar"}) {
		t.Fatal("a repeated entry changed the classpath")
	}
	if SameClasspath(a, []string{"libs/b.jar", "libs/a.jar"}) {
		t.Fatal("reordered classpath compared equal; the first entry declaring a class wins")
	}
}
