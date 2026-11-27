package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The LSP, MCP and daemon buffer analyzers judge a file the way the CLI
// does once they know the project root (#1834): sub/Foo.kt declaring app.sub
// in a project whose folder is named app is misplaced inside the project,
// whatever the checkout folder happens to be called.
func TestSingleFileAnalyzerUsesProjectRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte("rootProject.name = \"app\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sub", "Foo.kt")
	content := []byte("package app.sub\n\nclass Foo\n")
	reports := func(a *SingleFileAnalyzer) bool {
		t.Helper()
		cols, _, err := a.AnalyzeBufferColumns(context.Background(), path, content)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range cols.Findings() {
			if f.Rule == "InvalidPackageDeclaration" {
				return true
			}
		}
		return false
	}
	if reports(NewSingleFileAnalyzer(nil, nil)) {
		t.Fatal("without a root the absolute path ends in app/sub, as before; no finding expected")
	}
	rooted := NewSingleFileAnalyzer(nil, nil)
	rooted.SetProjectRootFor([]string{root})
	if !reports(rooted) {
		t.Fatal("with the project root set, sub/ does not hold package app.sub; InvalidPackageDeclaration expected")
	}
	unmarked := NewSingleFileAnalyzer(nil, nil)
	unmarked.SetProjectRootFor([]string{filepath.Join(root, "sub")}) // no marker there: Find walks up to root
	if !reports(unmarked) {
		t.Fatal("SetProjectRootFor must find the marked root above the given path")
	}
}
