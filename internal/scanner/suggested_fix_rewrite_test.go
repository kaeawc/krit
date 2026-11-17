package scanner

import (
	"strings"
	"testing"
)

func TestRewriteSuggestedFixTargets(t *testing.T) {
	cols := CollectFindings([]Finding{{
		File: "/repo/a.kt", Line: 1, Col: 1, Rule: "R", RuleSet: "s", Severity: "warning", Message: "m",
		SuggestedFixes: []SuggestedFix{{
			ID: "fix",
			Edits: []SuggestedEdit{
				{TargetFile: "/repo/krit.yml", Replacement: "x"},
				{Replacement: "same file"},
			},
		}},
	}})
	got := cols.RewriteSuggestedFixTargets(func(p string) string { return strings.TrimPrefix(p, "/repo/") })

	edits := got.SuggestedFixesAt(0)[0].Edits
	if edits[0].TargetFile != "krit.yml" {
		t.Errorf("TargetFile = %q, want krit.yml", edits[0].TargetFile)
	}
	if edits[1].TargetFile != "" {
		t.Errorf("an edit without a target gained %q", edits[1].TargetFile)
	}
	if orig := cols.SuggestedFixesAt(0)[0].Edits[0].TargetFile; orig != "/repo/krit.yml" {
		t.Errorf("receiver changed: %q", orig)
	}
}
