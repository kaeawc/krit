package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestReportPathFunc(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	base := reportBasePath("", []string{"app"})
	if base != filepath.Join(root, "app") {
		t.Fatalf("base = %q", base)
	}
	rel := ReportPathFunc(base, false)
	abs := ReportPathFunc(base, true)
	inside := filepath.Join(root, "app", "src", "A.kt")
	for _, spelling := range []string{inside, filepath.Join("app", "src", "A.kt")} {
		if got := rel(spelling); got != "src/A.kt" {
			t.Errorf("rel(%q) = %q, want src/A.kt", spelling, got)
		}
		if got := abs(spelling); got != filepath.ToSlash(inside) {
			t.Errorf("abs(%q) = %q, want %q", spelling, got, filepath.ToSlash(inside))
		}
	}
	outside := filepath.Join(root, "other", "B.kt")
	if got := rel(outside); got != filepath.ToSlash(outside) {
		t.Errorf("outside path = %q, want absolute %q", got, outside)
	}
}

// Every spelling of the scan path must produce the same reported paths.
func TestReportPathsIndependentOfScanPathSpelling(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "app"))
	build := func(files ...string) scanner.FindingColumns {
		b := scanner.NewFindingCollector(0)
		for _, f := range files {
			b.Append(scanner.Finding{File: f, Line: 1, Col: 1, RuleSet: "s", Rule: "R", Message: "m"})
		}
		return *b.Columns()
	}
	for _, scanPath := range []string{".", filepath.Join(root, "app")} {
		cols := build("A.kt", filepath.Join(root, "app", "res", "x.xml"))
		out := cols.RewriteFiles(ReportPathFunc(reportBasePath("", []string{scanPath}), false))
		got := []string{out.FileAt(0), out.FileAt(1)}
		if got[0] != "A.kt" || got[1] != "res/x.xml" {
			t.Errorf("scan path %q reported %v", scanPath, got)
		}
	}
}
