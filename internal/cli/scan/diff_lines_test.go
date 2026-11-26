package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestParseChangedLineIntervals(t *testing.T) {
	diff := `diff --git a/src/A.kt b/src/A.kt
index 1111111..2222222 100644
--- a/src/A.kt
+++ b/src/A.kt
@@ -10,0 +11,2 @@
+val added = 1
+val alsoAdded = 2
@@ -30,1 +32,1 @@
-val old = 1
+val changed = 2
`
	changed, err := parseChangedLineIntervals(diff, func(p string) string { return filepath.Join("/repo", p) })
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("/repo", "src/A.kt")
	intervals := changed[path]
	if len(intervals) != 2 {
		t.Fatalf("intervals = %+v, want 2", intervals)
	}
	if intervals[0] != (changedLineInterval{start: 11, end: 12}) {
		t.Fatalf("first interval = %+v", intervals[0])
	}
	if intervals[1] != (changedLineInterval{start: 32, end: 32}) {
		t.Fatalf("second interval = %+v", intervals[1])
	}
}

// git prints changed paths relative to the repository root. A scan run from a
// subdirectory must still resolve them to its own files; filepath.Abs from
// the working directory doubled the subdirectory and --diff dropped every
// finding.
func TestChangedFilesResolveFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=k@example.com", "-c", "user.name=k"}, args...)...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	file := filepath.Join(root, "app", "src", "main", "kotlin", "A.kt")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package a\n\nfun f() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "init")
	if err := os.WriteFile(file, []byte("package a\n\nfun f() = 1\n\nfun g() = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "app"))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(wd, "src", "main", "kotlin", "A.kt")
	files, err := getChangedFiles("HEAD", []string{wd})
	if err != nil {
		t.Fatal(err)
	}
	if !files[want] || len(files) != 1 {
		t.Fatalf("changed files = %v, want %s", files, want)
	}
	lines, err := getChangedLineIntervals("HEAD", []string{wd})
	if err != nil {
		t.Fatal(err)
	}
	if got := lines[want]; len(got) != 1 || got[0] != (changedLineInterval{start: 4, end: 5}) {
		t.Fatalf("changed lines = %v", lines)
	}
}

func TestFilterColumnsByChangedLines(t *testing.T) {
	file, _ := filepath.Abs("src/A.kt")
	columns := scanner.CollectFindings([]scanner.Finding{
		{File: file, Line: 10, Col: 1, Rule: "OldLine", RuleSet: "style", Severity: "warning", Message: "old"},
		{File: file, Line: 12, Col: 1, Rule: "NewLine", RuleSet: "style", Severity: "warning", Message: "new"},
	})
	filtered := filterColumnsByChangedLines(&columns, map[string][]changedLineInterval{
		file: {{start: 11, end: 12}},
	})
	if filtered.Len() != 1 {
		t.Fatalf("filtered.Len() = %d, want 1", filtered.Len())
	}
	if got := filtered.RuleAt(0); got != "NewLine" {
		t.Fatalf("remaining rule = %q", got)
	}
}
