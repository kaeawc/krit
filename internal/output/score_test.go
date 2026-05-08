package output

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestScoreFindings(t *testing.T) {
	report := ScoreFindings([]scanner.Finding{
		{Severity: "error"},
		{Severity: "warning"},
		{Severity: "warning"},
		{Severity: "info"},
	}, 12, 40)
	if report.Score != 121 {
		t.Fatalf("Score = %d, want 121", report.Score)
	}
	if report.Grade != "B" {
		t.Fatalf("Grade = %q, want B", report.Grade)
	}
	if report.FindingsBySeverity["error"] != 1 || report.FindingsBySeverity["warning"] != 2 || report.FindingsBySeverity["info"] != 1 {
		t.Fatalf("FindingsBySeverity = %#v", report.FindingsBySeverity)
	}
	if report.Summary.Files != 12 || report.Summary.Rules != 40 || report.Summary.Total != 4 {
		t.Fatalf("Summary = %#v", report.Summary)
	}
}

func TestGradeForScore(t *testing.T) {
	cases := map[int]string{
		0:     "A",
		100:   "A",
		101:   "B",
		1000:  "B",
		1001:  "C",
		5001:  "D",
		10001: "F",
	}
	for score, want := range cases {
		if got := GradeForScore(score); got != want {
			t.Fatalf("GradeForScore(%d) = %q, want %q", score, got, want)
		}
	}
}
