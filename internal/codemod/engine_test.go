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
	dry, err := Run(root, recipe, false)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Matches != 1 || dry.EditsApplied != 0 {
		t.Fatalf("dry run = %+v, want one unapplied match", dry)
	}
	applied, err := Run(root, recipe, true)
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
