package scanner

import (
	"path/filepath"
	"testing"
)

// ApplyDelta and FilterColumnsByFilePaths match a file however it is
// spelled: the CLI's absolute rows and the daemon's relative ones (#1832).
func TestDeltaMatchesFilesAcrossSpellings(t *testing.T) {
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	t.Chdir(dir)
	abs := filepath.Join(dir, "src", "A.kt")
	rel := filepath.Join("src", "A.kt")
	other := filepath.Join(dir, "src", "B.kt")
	prior := CollectFindings([]Finding{
		{File: abs, Line: 3, Rule: "R", Message: "old"},
		{File: other, Line: 1, Rule: "R", Message: "kept"},
	})
	replacement := CollectFindings([]Finding{{File: rel, Line: 3, Rule: "R", Message: "new"}})
	merged := ApplyDelta(&prior, &replacement, []string{rel})
	var msgs []string
	for _, f := range merged.Findings() {
		msgs = append(msgs, f.Message)
	}
	if len(msgs) != 2 || msgs[0] == "old" || msgs[1] == "old" {
		t.Fatalf("merged = %v, want the absolute prior row of A.kt replaced and B.kt kept", msgs)
	}
	if got := FilterColumnsByFilePaths(&prior, map[string]bool{rel: true}); got.Len() != 1 {
		t.Fatalf("a relative allowed path kept %d rows of the absolute A.kt, want 1", got.Len())
	}
}
