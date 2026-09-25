package deadcode

import (
	"path/filepath"
	"testing"
)

func TestTopLevelAnnotationDeadCodeRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main", "kotlin", "Roots.kt"), `package p
@Provides("binding")
fun provide() = 1
fun next() {}
`)
	findings, err := AnalyzeProject(dir, ProjectOptions{Paths: []string{dir}, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := findFinding(findings, "provide"); got != nil {
		t.Errorf("@Provides should be a reachability root, got %+v", got)
	}
}
