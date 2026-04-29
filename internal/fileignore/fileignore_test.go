package fileignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPrunedDir(t *testing.T) {
	for _, name := range []string{".git", "build", "node_modules", ".gradle", "target", "vendor"} {
		if !DefaultPrunedDir(name) {
			t.Fatalf("DefaultPrunedDir(%q) = false, want true", name)
		}
	}
	if DefaultPrunedDir("src") {
		t.Fatal("DefaultPrunedDir(\"src\") = true, want false")
	}
}

func TestMatcherRespectsRootAndNestedGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".claude/worktrees/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(root, "module")
	if err := os.MkdirAll(filepath.Join(module, "generated"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, ".gitignore"), []byte("generated/\n*.kt\n!Keep.kt\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	matcher := MatcherForPath(root, info, nil)

	if !matcher.Ignored(filepath.Join(root, ".claude", "worktrees"), true) {
		t.Fatal("expected root .gitignore to ignore .claude/worktrees")
	}
	if !matcher.Ignored(filepath.Join(module, "generated", "Ignored.kt"), false) {
		t.Fatal("expected nested .gitignore to ignore generated file")
	}
	if !matcher.Ignored(filepath.Join(module, "sub", "Ignored.kt"), false) {
		t.Fatal("expected nested basename pattern to apply below its directory")
	}
	if matcher.Ignored(filepath.Join(module, "sub", "Keep.kt"), false) {
		t.Fatal("expected nested negation to override parent basename pattern")
	}
	if matcher.Ignored(filepath.Join(module, "src", "Keep.kt"), false) {
		t.Fatal("expected non-ignored source file to be kept")
	}
}

func TestMatcherFindsGitFileRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /tmp/worktree.git\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("generated/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "src", "main", "kotlin")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(child)
	if err != nil {
		t.Fatal(err)
	}

	matcher := MatcherForPath(child, info, nil)
	if !matcher.Ignored(filepath.Join(root, "generated", "Ignored.kt"), false) {
		t.Fatal("expected matcher rooted by .git file to apply root .gitignore")
	}
}
