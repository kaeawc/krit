package gradlemodel

import (
	"path/filepath"
	"testing"
)

func TestProjectIndexMapsFilesToDeepestProject(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "shared", "src")
	model := &Model{Projects: []Project{
		{Path: ":app", Dir: filepath.Join(root, "app")},
		{Path: ":lib", Dir: filepath.Join(root, "lib"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", SourceDirs: []string{shared}},
		}},
		{Path: ":lib:nested", Dir: filepath.Join(root, "lib", "nested")},
	}}
	idx := model.ProjectIndex()
	cases := map[string]string{
		filepath.Join(root, "app", "src", "main", "kotlin", "A.kt"):           ":app",
		filepath.Join(root, "lib", "src", "test", "kotlin", "B.kt"):           ":lib",
		filepath.Join(root, "lib", "nested", "src", "main", "kotlin", "C.kt"): ":lib:nested",
		filepath.Join(shared, "D.kt"):                                         ":lib",
		filepath.Join(root, "outside", "E.kt"):                                "",
	}
	for path, want := range cases {
		if got := idx.ProjectOf(path); got != want {
			t.Errorf("ProjectOf(%s) = %q, want %q", path, got, want)
		}
	}
}

func TestProjectIndexNilModel(t *testing.T) {
	var model *Model
	if got := model.ProjectIndex().ProjectOf("/x/A.kt"); got != "" {
		t.Fatalf("nil model mapped a file to %q", got)
	}
}
