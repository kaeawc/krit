package fileignore

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDocSamplePath(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root,
		"build.gradle.kts",
		"docs/plugin/references/examples/Sample.kt",
		"docs/plugin/references/Notes.kt",
		"lib/build.gradle.kts",
		"lib/examples/Sample.kt",
		"app/src/main/kotlin/com/acme/references/Real.kt",
		"demo/examples/app/build.gradle.kts",
		"demo/examples/app/Main.kt",
		"plain/Main.kt",
	)
	cases := map[string]bool{
		"docs/plugin/references/examples/Sample.kt":       true,
		"docs/plugin/references/Notes.kt":                 true,
		"lib/examples/Sample.kt":                          false, // beside a module build file
		"app/src/main/kotlin/com/acme/references/Real.kt": false, // inside a source tree
		"demo/examples/app/Main.kt":                       false, // example project with its own build file
		"plain/Main.kt":                                   false,
	}
	for rel, want := range cases {
		if got := DocSamplePath(filepath.Join(root, rel)); got != want {
			t.Errorf("DocSamplePath(%s) = %v, want %v", rel, got, want)
		}
	}
}

func TestDocSamplePath_NoGitCheckoutIsNeverASample(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "docs/examples/Sample.kt")
	if DocSamplePath(filepath.Join(root, "docs/examples/Sample.kt")) {
		t.Error("a path outside a Git checkout must be analysed")
	}
}
