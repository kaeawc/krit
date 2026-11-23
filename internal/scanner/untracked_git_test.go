package scanner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// Regression for #1059: a new file that is not `git add`-ed yet was
// silently skipped because collection listed only tracked files.
func TestCollectKotlinAndJavaFilesIncludesUntrackedNotIgnored(t *testing.T) {
	if !scannerGitAvailable(t) {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	write := func(rel, content string) string {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	tracked := write("src/main/kotlin/Tracked.kt", "class Tracked\n")
	write(".gitignore", "build/\n")
	if out, err := exec.Command("git", "-C", root, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	untracked := write("src/main/kotlin/tmp/Bad.kt", "class Bad\n")
	untrackedJava := write("src/main/java/New.java", "class New {}\n")
	write("build/generated/Ignored.kt", "class Ignored\n")

	kotlin, java, err := CollectKotlinAndJavaFiles(context.Background(), []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(kotlin)
	want := []string{tracked, untracked}
	sort.Strings(want)
	if len(kotlin) != 2 || kotlin[0] != want[0] || kotlin[1] != want[1] {
		t.Errorf("Kotlin files = %v, want %v", kotlin, want)
	}
	if len(java) != 1 || java[0] != untrackedJava {
		t.Errorf("Java files = %v, want [%s]", java, untrackedJava)
	}
}
