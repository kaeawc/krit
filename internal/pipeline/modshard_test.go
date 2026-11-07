package pipeline

import (
	"path/filepath"
	"testing"
)

func TestUnderSourceDirsKeepsOnlySourceSetFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "lib", "src", "main", "kotlin")
	files := []string{
		filepath.Join(dir, "a", "A.kt"),
		filepath.Join(dir, "B.kt"),
		filepath.Join(root, "lib", "build.gradle.kts"),
		filepath.Join(root, "tools", "script.kts"),
		filepath.Join(dir, "testData", "C.kt"),
	}
	got := underSourceDirs(files, []string{dir})
	if len(got) != 2 || got[0] != files[0] || got[1] != files[1] {
		t.Fatalf("got %v", got)
	}
}
