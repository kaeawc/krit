package scan

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestFilterColumnsNewSince(t *testing.T) {
	columns := scanner.CollectFindings([]scanner.Finding{
		{File: "/repo/src/A.kt", Line: 1, Col: 1, Rule: "RuleA", RuleSet: "style", Severity: "warning", Message: "old"},
		{File: "/repo/src/B.kt", Line: 2, Col: 1, Rule: "RuleB", RuleSet: "style", Severity: "warning", Message: "new"},
	})
	baseIDs := map[string]bool{
		deltaFindingID("src/A.kt", "RuleA", "old"): true,
	}
	filtered := filterColumnsNewSince(&columns, baseIDs, "/repo")
	if filtered.Len() != 1 {
		t.Fatalf("filtered.Len() = %d, want 1", filtered.Len())
	}
	if got := filtered.RuleAt(0); got != "RuleB" {
		t.Fatalf("remaining rule = %q", got)
	}
}

func TestMapPathToWorktree(t *testing.T) {
	got := mapPathToWorktree("/repo", "/tmp/base", "/repo/app/src")
	if got != "/tmp/base/app/src" {
		t.Fatalf("mapPathToWorktree() = %q", got)
	}
}
