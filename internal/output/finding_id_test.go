package output

import "testing"

func TestFindingID_deterministicShape(t *testing.T) {
	f := JSONFinding{Rule: "R", File: "src/foo/Bar.kt", Line: 12, Column: 4}
	want := "R:src/foo/Bar.kt:12:4"
	if got := FindingID(f); got != want {
		t.Errorf("FindingID: got %q want %q", got, want)
	}
}
