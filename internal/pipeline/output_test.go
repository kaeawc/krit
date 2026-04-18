package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

// twoFindingsInput builds a minimal OutputInput with two synthetic
// findings spanning distinct rules. It is the shared fixture for the
// per-format happy-path tests.
func twoFindingsInput(t *testing.T, format string) (OutputInput, *bytes.Buffer) {
	t.Helper()
	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:     "/tmp/foo.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "RuleX",
			Severity: "warning",
			Message:  "trouble brewing",
		},
		{
			File:     "/tmp/bar.kt",
			Line:     2,
			Col:      3,
			RuleSet:  "style",
			Rule:     "RuleY",
			Severity: "error",
			Message:  "definitely wrong",
		},
	})

	var buf bytes.Buffer
	in := OutputInput{
		FixupResult: FixupResult{
			CrossFileResult: CrossFileResult{
				DispatchResult: DispatchResult{
					IndexResult: IndexResult{
						ParseResult: ParseResult{
							Paths: []string{"/tmp"},
						},
					},
					Findings: columns,
				},
			},
		},
		Writer:    &buf,
		Format:    format,
		StartTime: time.Now().Add(-10 * time.Millisecond),
		Version:   "test-v0",
	}
	return in, &buf
}

func TestOutputPhase_Name(t *testing.T) {
	if got := (OutputPhase{}).Name(); got != "output" {
		t.Fatalf("Name() = %q, want %q", got, "output")
	}
}

func TestOutputPhase_Run_JSON(t *testing.T) {
	in, buf := twoFindingsInput(t, "json")
	if _, err := (OutputPhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"findings"`) {
		t.Errorf("output missing findings key; got:\n%s", out)
	}
	if !strings.Contains(out, `"success": false`) {
		t.Errorf("output missing success=false; got:\n%s", out)
	}
	if !strings.Contains(out, "RuleX") || !strings.Contains(out, "RuleY") {
		t.Errorf("output missing rule names; got:\n%s", out)
	}
}

func TestOutputPhase_Run_Plain(t *testing.T) {
	in, buf := twoFindingsInput(t, "plain")
	if _, err := (OutputPhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "RuleX") || !strings.Contains(out, "RuleY") {
		t.Errorf("plain output missing rule names; got:\n%s", out)
	}
}

func TestOutputPhase_Run_SARIF(t *testing.T) {
	in, buf := twoFindingsInput(t, "sarif")
	if _, err := (OutputPhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &top); err != nil {
		t.Fatalf("sarif output is not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := top["version"]; !ok {
		t.Errorf("sarif output missing top-level 'version'")
	}
	if _, ok := top["runs"]; !ok {
		t.Errorf("sarif output missing top-level 'runs'")
	}
}

func TestOutputPhase_Run_Checkstyle(t *testing.T) {
	in, buf := twoFindingsInput(t, "checkstyle")
	if _, err := (OutputPhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "<?xml") {
		t.Errorf("checkstyle output should start with <?xml; got: %q", out[:min(len(out), 40)])
	}
	if !strings.Contains(out, "<checkstyle") {
		t.Errorf("checkstyle output missing <checkstyle element; got:\n%s", out)
	}
}

func TestOutputPhase_Run_UnknownFormat(t *testing.T) {
	in, _ := twoFindingsInput(t, "xml")
	_, err := (OutputPhase{}).Run(context.Background(), in)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("error message = %q, want to contain 'unknown format'", err.Error())
	}
}

func TestOutputPhase_Run_BaselineFilter(t *testing.T) {
	// Build a detekt-format baseline XML suppressing "RuleX". The
	// BaselineID for a finding is "Rule:filename:signature"; we pass
	// an empty signature (matches WriteBaseline's output).
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "baseline.xml")
	// RuleX is on /tmp/foo.kt with message "trouble brewing".
	// BaselineID format is "RuleName:filename:signature" (see
	// baselineIDParts in scanner/baseline.go); when the rule does not
	// supply a signature it defaults to "$RuleName$Message". With
	// basePath="" we get filename-only IDs.
	baselineXML := `<?xml version="1.0" ?>
<SmellBaseline>
  <ManuallySuppressedIssues>
    <ID>RuleX:foo.kt:$RuleX$trouble brewing</ID>
  </ManuallySuppressedIssues>
  <CurrentIssues></CurrentIssues>
</SmellBaseline>`
	if err := os.WriteFile(baselinePath, []byte(baselineXML), 0644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	in, buf := twoFindingsInput(t, "json")
	in.BaselinePath = baselinePath
	// basePath "" → BaselineID uses filename only, matching our XML.
	in.BasePath = ""
	// Clear the Paths fallback so basePath stays "" and the compat
	// (filename-only) ID path is used.
	in.FixupResult.Paths = nil

	res, err := (OutputPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "RuleX") {
		t.Errorf("baseline did not suppress RuleX; output:\n%s", out)
	}
	if !strings.Contains(out, "RuleY") {
		t.Errorf("baseline over-filtered — RuleY missing; output:\n%s", out)
	}
	if res.FinalFindings.Len() != 1 {
		t.Errorf("FinalFindings.Len() = %d, want 1", res.FinalFindings.Len())
	}
}

func TestOutputPhase_Run_EmptyFindings_SuccessTrue(t *testing.T) {
	empty := scanner.FindingColumns{}
	var buf bytes.Buffer
	in := OutputInput{
		FixupResult: FixupResult{
			CrossFileResult: CrossFileResult{
				DispatchResult: DispatchResult{
					Findings: empty,
				},
			},
		},
		Writer:    &buf,
		Format:    "json",
		StartTime: time.Now(),
		Version:   "test-v0",
	}
	if _, err := (OutputPhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"success": true`) {
		t.Errorf("empty findings should yield success=true; got:\n%s", out)
	}
}

func TestOutputPhase_Run_WarningsAsErrors(t *testing.T) {
	in, buf := twoFindingsInput(t, "json")
	in.WarningsAsErrors = true

	res, err := (OutputPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	for row := 0; row < res.FinalFindings.Len(); row++ {
		if sev := res.FinalFindings.SeverityAt(row); sev != "error" {
			t.Errorf("row %d severity = %q, want error", row, sev)
		}
	}
	out := buf.String()
	if strings.Contains(out, `"severity": "warning"`) {
		t.Errorf("warnings-as-errors should have removed warning severity from JSON; got:\n%s", out)
	}
}

func TestOutputPhase_Run_MinConfidenceFilter(t *testing.T) {
	// Build two findings: a high-confidence RuleA and a low-confidence
	// RuleB. Setting MinConfidence=0.5 should drop only RuleB.
	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:       "/tmp/foo.kt",
			Line:       1,
			Col:        1,
			RuleSet:    "style",
			Rule:       "RuleA",
			Severity:   "warning",
			Message:    "high confidence",
			Confidence: 0.9,
		},
		{
			File:       "/tmp/bar.kt",
			Line:       2,
			Col:        3,
			RuleSet:    "style",
			Rule:       "RuleB",
			Severity:   "warning",
			Message:    "low confidence",
			Confidence: 0.2,
		},
	})

	var buf bytes.Buffer
	in := OutputInput{
		FixupResult: FixupResult{
			CrossFileResult: CrossFileResult{
				DispatchResult: DispatchResult{
					Findings: columns,
				},
			},
		},
		Writer:        &buf,
		Format:        "json",
		StartTime:     time.Now(),
		Version:       "test-v0",
		MinConfidence: 0.5,
	}

	res, err := (OutputPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.FinalFindings.Len() != 1 {
		t.Fatalf("FinalFindings.Len() = %d, want 1", res.FinalFindings.Len())
	}
	out := buf.String()
	if !strings.Contains(out, "RuleA") {
		t.Errorf("min-confidence should have kept RuleA; got:\n%s", out)
	}
	if strings.Contains(out, "RuleB") {
		t.Errorf("min-confidence should have dropped RuleB; got:\n%s", out)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
