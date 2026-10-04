package scanner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func scannerGitAvailable(t *testing.T) bool {
	t.Helper()
	_, err := exec.LookPath("git")
	return err == nil
}

func TestTrackedIdeaTemplateExcludedFromSourceCollection(t *testing.T) {
	if !scannerGitAvailable(t) {
		t.Skip("git not on PATH")
	}
	root := filepath.Join(t.TempDir(), ".claude", "worktrees", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	keep := filepath.Join(root, "src", "App.kt")
	template := filepath.Join(root, ".idea", "fileTemplates", "Class.kt")
	for _, path := range []string{keep, template} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("class Example {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", root, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	if paths, ok := gitLsKotlinJava(context.Background(), root); !ok || len(paths) != 2 {
		t.Fatalf("expected git fast path with tracked template, got %v (ok=%t)", paths, ok)
	}
	kotlin, java, err := CollectKotlinAndJavaFiles(context.Background(), []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(kotlin) != 1 || kotlin[0] != keep || len(java) != 0 {
		t.Fatalf("collected Kotlin=%v Java=%v, want only %s", kotlin, java, keep)
	}
	plain, err := CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != 1 || plain[0] != keep {
		t.Fatalf("walked files = %v", plain)
	}
	explicit, err := CollectKotlinFiles([]string{template}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(explicit) != 0 {
		t.Fatalf("explicit template files = %v", explicit)
	}
}
