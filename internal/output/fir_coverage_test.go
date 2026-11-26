package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestFirCoverageLineAndRatio(t *testing.T) {
	if line := FirCoverageLine(FirCoverage{Authoritative: 5}); line != "" {
		t.Fatalf("a scan with nothing gated prints no line, got %q", line)
	}
	if r := (FirCoverage{}).Ratio(); r != 1 {
		t.Fatalf("empty coverage ratio = %v, want 1", r)
	}
	c := FirCoverage{Authoritative: 3875, Gated: 23338, TopCauses: []FirGatingCause{
		{Cause: "Unresolved reference 'androidx.compose'", Count: 12034},
		{Cause: "Unresolved reference 'dagger'", Count: 4000},
		{Cause: "compiler crash", Count: 3},
		{Cause: "never listed", Count: 1},
	}}
	line := FirCoverageLine(c)
	for _, want := range []string{
		"FIR analyzed 3,875 of 27,213 files (14%); 23,338 gated by compile errors",
		"top causes: Unresolved reference 'androidx.compose' x12,034, Unresolved reference 'dagger' x4,000, compiler crash x3)",
		"Run with -v for the list.",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "never listed") {
		t.Errorf("line names more than %d causes: %q", maxCausesInLine, line)
	}
}

// PR mode (#1660) says how much it left to Go, gated files or not.
func TestFirCoverageLineInPRMode(t *testing.T) {
	if line, want := FirCoverageLine(FirCoverage{Authoritative: 3, OutsidePR: 27210}),
		"FIR analyzed 3 of 3 changed files (PR mode: 27,210 unchanged files keep Go's verdict)."; line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
	line := FirCoverageLine(FirCoverage{Authoritative: 2, Gated: 1, OutsidePR: 10})
	for _, want := range []string{"FIR analyzed 2 of 3 files (67%); 1 gated by compile errors", "(PR mode: 10 unchanged files keep Go's verdict)"} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q lacks %q", line, want)
		}
	}
}

func TestFirCoverageInJSONAndSARIF(t *testing.T) {
	cov := &FirCoverage{Authoritative: 2, Gated: 1, Excluded: 4, TopCauses: []FirGatingCause{{Cause: "Unresolved reference 'x'", Count: 1}}}
	var cols scanner.FindingColumns
	var buf bytes.Buffer
	if err := FormatJSONColumnsWithOptions(&buf, &cols, "v", 0, 0, time.Time{}, nil, nil, nil, nil, nil, nil, JSONOptions{FirCoverage: cov}); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Summary struct {
			Fir *FirCoverage `json:"fir"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil || report.Summary.Fir == nil || len(report.Summary.Fir.TopCauses) != 1 || report.Summary.Fir.Gated != 1 {
		t.Fatalf("summary.fir = %+v (%v)\n%s", report.Summary.Fir, err, buf.String())
	}
	buf.Reset()
	if err := FormatJSONColumnsWithOptions(&buf, &cols, "v", 0, 0, time.Time{}, nil, nil, nil, nil, nil, nil, JSONOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"fir"`) {
		t.Fatalf("summary.fir present without coverage:\n%s", buf.String())
	}

	buf.Reset()
	if err := FormatSARIFColumnsReport(&buf, &cols, "v", SARIFOptions{FirCoverage: cov}); err != nil {
		t.Fatal(err)
	}
	var sarif struct {
		Runs []struct {
			Properties struct {
				Fir *FirCoverage `json:"fir"`
			} `json:"properties"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil || len(sarif.Runs) != 1 || sarif.Runs[0].Properties.Fir == nil || sarif.Runs[0].Properties.Fir.Authoritative != 2 {
		t.Fatalf("runs[0].properties.fir missing (%v)\n%s", err, buf.String())
	}
	buf.Reset()
	if err := FormatSARIFColumns(&buf, &cols, "v"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"properties":{"fir"`) {
		t.Fatalf("SARIF carries fir properties without coverage:\n%s", buf.String())
	}
}

// Withheld FIR-required findings are reported even when no gated file is
// counted, so they never disappear without a line (#1675).
func TestFirCoverageLineReportsWithheldFindings(t *testing.T) {
	line := FirCoverageLine(FirCoverage{Authoritative: 3, Withheld: 2})
	if !strings.Contains(line, "except 2 withheld for FIR-required rules") {
		t.Fatalf("line %q does not report the withheld findings", line)
	}
	if got := FirCoverageLine(FirCoverage{Authoritative: 3}); got != "" {
		t.Fatalf("nothing gated or withheld: got %q", got)
	}
}

// In PR mode with nothing gated, withheld findings still get their clause
// beside the PR-mode note instead of the short changed-files line.
func TestFirCoverageLineReportsWithheldFindingsInPRMode(t *testing.T) {
	line := FirCoverageLine(FirCoverage{Authoritative: 3, Withheld: 2, OutsidePR: 40})
	for _, want := range []string{"except 2 withheld for FIR-required rules", "PR mode: 40 unchanged files keep Go's verdict"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q lacks %q", line, want)
		}
	}
}
