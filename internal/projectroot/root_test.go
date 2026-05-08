package projectroot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWalksUpToGitRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "app", "src", "main")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := Find([]string{nested}); got != root {
		t.Fatalf("Find() = %q, want %q", got, root)
	}
}

func TestFindUsesFileParentBeforeWalking(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "app", "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "Example.kt")
	if err := os.WriteFile(file, []byte("fun example() = Unit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := Find([]string{file}); got != root {
		t.Fatalf("Find() = %q, want %q", got, root)
	}
}

func TestFindFallsBackToStartingDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := Find([]string{nested}); got != nested {
		t.Fatalf("Find() = %q, want %q", got, nested)
	}
}
