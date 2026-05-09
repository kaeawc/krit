package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignoreEntryCreatesFile(t *testing.T) {
	repo := t.TempDir()
	added, err := EnsureGitignoreEntry(repo, SnapshotsGitignorePattern)
	if err != nil {
		t.Fatalf("EnsureGitignoreEntry: %v", err)
	}
	if !added {
		t.Fatal("expected added=true on missing .gitignore")
	}
	data, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), SnapshotsGitignorePattern) {
		t.Fatalf("missing pattern: %q", data)
	}
}

func TestEnsureGitignoreEntryIdempotent(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, ".gitignore")
	if err := os.WriteFile(path, []byte("build/\n.krit/snapshots/\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	added, err := EnsureGitignoreEntry(repo, SnapshotsGitignorePattern)
	if err != nil {
		t.Fatalf("EnsureGitignoreEntry: %v", err)
	}
	if added {
		t.Fatal("expected added=false when pattern already present")
	}
	data, _ := os.ReadFile(path)
	if strings.Count(string(data), SnapshotsGitignorePattern) != 1 {
		t.Fatalf("pattern duplicated: %q", data)
	}
}

func TestEnsureGitignoreEntryEquivalentTrailingSlash(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, ".gitignore")
	// Pre-existing entry without trailing slash should still count.
	if err := os.WriteFile(path, []byte(".krit/snapshots\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	added, err := EnsureGitignoreEntry(repo, SnapshotsGitignorePattern)
	if err != nil {
		t.Fatalf("EnsureGitignoreEntry: %v", err)
	}
	if added {
		t.Fatal("expected slash-equivalent pattern to be detected")
	}
}

func TestEnsureGitignoreEntryAppendsWithNewline(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, ".gitignore")
	// Existing file without trailing newline.
	if err := os.WriteFile(path, []byte("build/"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := EnsureGitignoreEntry(repo, SnapshotsGitignorePattern); err != nil {
		t.Fatalf("EnsureGitignoreEntry: %v", err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "build/\n.krit/snapshots/\n") {
		t.Fatalf("expected newline-separated entries, got %q", got)
	}
}

func TestEnsureGitignoreIgnoresCommentLines(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, ".gitignore")
	if err := os.WriteFile(path, []byte("# .krit/snapshots/\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	added, err := EnsureGitignoreEntry(repo, SnapshotsGitignorePattern)
	if err != nil {
		t.Fatalf("EnsureGitignoreEntry: %v", err)
	}
	if !added {
		t.Fatal("commented-out pattern should not count as present")
	}
}

func TestInstallHookAppendsGitignoreEntry(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := InstallHook(repo, false); err != nil {
		t.Fatalf("InstallHook: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("expected .gitignore to be created: %v", err)
	}
	if !strings.Contains(string(data), SnapshotsGitignorePattern) {
		t.Fatalf("missing pattern in .gitignore: %q", data)
	}
}
