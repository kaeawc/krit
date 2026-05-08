package scan

import (
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
	changed, err := parseChangedLineIntervals(diff)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := filepath.Abs("src/A.kt")
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
