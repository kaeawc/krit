package serve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/module"
)

func writeKt(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCountKotlinFilesKeepsBuildPackageInSourceRoot(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "app", "src", "main", "kotlin")
	writeKt(t, filepath.Join(src, "com", "example", "A.kt"))
	writeKt(t, filepath.Join(src, "com", "example", "build", "B.kt"))
	writeKt(t, filepath.Join(root, "app", "build", "generated", "C.kt"))

	graph := &module.Graph{Modules: map[string]*module.Module{
		":app": {Path: ":app", Dir: filepath.Join(root, "app"), SourceRoots: []string{src}},
	}}
	if got, scoped := countKotlinFiles(root, graph); got != 2 || !scoped {
		t.Errorf("module count = %d (scoped %v), want 2 scoped (the build package inside the source root counts)", got, scoped)
	}
	// The repo-root fallback skips every build directory, including build output.
	if got, scoped := countKotlinFiles(root, nil); got != 1 || scoped {
		t.Errorf("root fallback count = %d (scoped %v), want 1 unscoped", got, scoped)
	}
}
