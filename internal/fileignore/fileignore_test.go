package fileignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPrunedDir(t *testing.T) {
	// Hard-coded prune list: caches/metadata that .gitignore can't
	// be relied on to cover.
	pruned := []string{
		".git",
		".krit", ".krit-cache", ".krit-types",
		".gradle", ".idea", ".kotlin",
		".claude", ".codex", ".grit",
	}
	for _, name := range pruned {
		if !DefaultPrunedDir(name) {
			t.Fatalf("DefaultPrunedDir(%q) = false, want true", name)
		}
	}
	// Project-output / dep dirs are deliberately NOT in the hard-
	// coded list — projects that ignore them in `.gitignore` (the
	// overwhelming convention) get them pruned via the matcher.
	// Hard-coding them here would over-prune projects that
	// intentionally check those names in.
	for _, name := range []string{"src", "build", "node_modules", "target", "vendor", "out", "external"} {
		if DefaultPrunedDir(name) {
			t.Fatalf("DefaultPrunedDir(%q) = true, want false", name)
		}
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
