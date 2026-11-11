package codemod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDryRunAndApply(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src", "main", "kotlin", "Example.kt")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`
package test

fun log() {
    Timber.d("hello")
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	recipe := Recipe{
		Name:        "replace-timber",
		Language:    "kotlin",
		Match:       `((simple_identifier) @match (#eq? @match "Timber"))`,
		Replacement: "logger",
	}
	dry, err := Run(t.Context(), root, recipe, false)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Matches != 1 || dry.EditsApplied != 0 {
		t.Fatalf("dry run = %+v, want one unapplied match", dry)
	}
	applied, err := Run(t.Context(), root, recipe, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.EditsApplied != 1 || applied.FilesModified != 1 {
		t.Fatalf("apply = %+v, want one applied edit", applied)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `logger.d("hello")`) {
		t.Fatalf("replacement not applied:\n%s", content)
	}
}

func TestRunApplyKeepsModeAndLeavesNoTempFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Example.kt")
	if err := os.WriteFile(path, []byte("fun log() { Timber.d(\"x\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recipe := Recipe{
		Name:        "replace-timber",
		Language:    "kotlin",
		Match:       `((simple_identifier) @match (#eq? @match "Timber"))`,
		Replacement: "logger",
	}
	if _, err := Run(t.Context(), root, recipe, true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600 preserved", info.Mode().Perm())
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("leftover files after apply: %v", entries)
	}
}

func TestApplyEditsWritesThroughSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "Real.kt")
	if err := os.WriteFile(target, []byte("val a = Timber\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "Link.kt")
	if err := os.Symlink("Real.kt", link); err != nil {
		t.Skip("symlinks unavailable")
	}
	start := strings.Index("val a = Timber\n", "Timber")
	edit := Edit{File: link, StartByte: start, EndByte: start + len("Timber"), Replacement: "logger"}
	if _, modified, err := ApplyEdits([]Edit{edit}); err != nil || modified != 1 {
		t.Fatalf("ApplyEdits = %d, %v", modified, err)
	}
	if got, _ := os.ReadFile(target); string(got) != "val a = logger\n" {
		t.Fatalf("target = %q, want edited", got)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced by a regular file: %v", err)
	}
}
