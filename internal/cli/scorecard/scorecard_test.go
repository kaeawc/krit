package scorecard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestRowsFromData(t *testing.T) {
	root := t.TempDir()
	coreDir := filepath.Join(root, "core")
	graph := module.NewModuleGraph(root)
	graph.Modules[":core"] = &module.Module{Path: ":core", Dir: coreDir}
	mainFile := &scanner.File{
		Path:  filepath.Join(coreDir, "src", "main", "kotlin", "Core.kt"),
		Lines: make([]string, 1000),
	}
	testFile := &scanner.File{
		Path:  filepath.Join(coreDir, "src", "test", "kotlin", "CoreTest.kt"),
		Lines: make([]string, 500),
	}
	rows := rowsFromData(graph, map[string][]*scanner.File{":core": {mainFile, testFile}}, []scanner.Finding{
		{File: mainFile.Path, Rule: "MagicNumber"},
		{File: mainFile.Path, Rule: "LongMethod"},
	})
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].FindingsPerKLOC != 2.0 {
		t.Fatalf("FindingsPerKLOC = %.1f", rows[0].FindingsPerKLOC)
	}
	if rows[0].TestRatio != 0.5 {
		t.Fatalf("TestRatio = %.1f", rows[0].TestRatio)
	}
}

func TestWriteMarkdown(t *testing.T) {
	var buf bytes.Buffer
	WriteMarkdown(&buf, []Row{{Module: ":core", FindingsPerKLOC: 2.3, AvgComplexity: 4.1, TestRatio: 1.2}})
	got := buf.String()
	if !strings.Contains(got, "| :core | 2.3 | 4.1 | 1.2 |") {
		t.Fatalf("markdown = %q", got)
	}
}

func TestBuildScorecard(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte(`include(":core")`), 0644); err != nil {
		t.Fatal(err)
	}
	mainDir := filepath.Join(root, "core", "src", "main", "kotlin")
	testDir := filepath.Join(root, "core", "src", "test", "kotlin")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainDir, "Core.kt"), []byte("package core\nfun core(x: Int) = if (x > 0) x else -x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "CoreTest.kt"), []byte("package core\nfun testCore() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	rows, err := Build([]string{root}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Module != ":core" {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[0].AvgComplexity == 0 {
		t.Fatalf("AvgComplexity = 0, want complexity from function AST")
	}
}
