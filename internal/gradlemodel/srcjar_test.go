package gradlemodel

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A source set's generatedSourceJars are unpacked by Load and become
// generated source roots, so every model consumer (compile context, module
// specs, generated-path filters) sees them without knowing about srcjars.
func TestLoadUnpacksGeneratedSourceJars(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KRIT_HOME", filepath.Join(root, "krit-home"))
	projectDir := filepath.Join(root, "app")
	mainDir := filepath.Join(projectDir, "src", "main", "kotlin")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(root, "ksp.srcjar")
	f, err := os.Create(jar)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("gen/Foo.kt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("package gen\nclass Foo\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	modelDir := filepath.Join(root, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"schema": 2, "projects": []map[string]any{{
		"path": ":app", "dir": projectDir, "sourceSets": []map[string]any{{
			"name": "main", "kind": "main", "sourceDirs": []string{mainDir},
			"generatedSourceJars": []string{jar, filepath.Join(root, "missing.srcjar")},
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "model.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	model, warnings, err := Load(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "missing.srcjar") || !strings.Contains(warnings[0], "source set main") {
		t.Fatalf("warnings = %v, want one naming the missing srcjar and its source set", warnings)
	}
	set := model.Projects[0].SourceSets[0]
	if len(set.GeneratedSourceDirs) != 1 {
		t.Fatalf("GeneratedSourceDirs = %v, want the unpacked srcjar root", set.GeneratedSourceDirs)
	}
	if _, err := os.Stat(filepath.Join(set.GeneratedSourceDirs[0], "gen", "Foo.kt")); err != nil {
		t.Fatalf("unpacked root lacks gen/Foo.kt: %v", err)
	}
	if got := model.SourceDirs(); len(got) != 2 || got[1] != set.GeneratedSourceDirs[0] {
		t.Fatalf("SourceDirs() = %v, want main then the srcjar root", got)
	}
}
