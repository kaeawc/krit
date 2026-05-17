package applysuggestion

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/fixer"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

func reportFile(t *testing.T, report output.JSONReport) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	return path
}

func TestList_printsFindingsAndSuggestions(t *testing.T) {
	report := output.JSONReport{
		Findings: []output.JSONFinding{
			{
				File:    "a.kt",
				Line:    5,
				Column:  3,
				RuleSet: "style",
				Rule:    "PreferVal",
				Message: "use val",
				SuggestedFixes: []output.JSONSuggestedFix{
					{ID: "use-val", Title: "Convert to val",
						Edits: []output.JSONSuggestedEdit{{StartLine: 5, EndLine: 5, Replacement: "val x = 1"}}},
					{ID: "explain", Title: "Explain"},
				},
			},
			{File: "b.kt", Rule: "NoSuggestions", Line: 1, Column: 1},
		},
	}
	path := reportFile(t, report)
	var stdout, stderr bytes.Buffer
	rc := run([]string{"--list", path}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	out := stdout.String()
	wantID := "PreferVal:a.kt:5:3"
	if !strings.Contains(out, wantID) {
		t.Errorf("output missing finding id %q: %s", wantID, out)
	}
	if !strings.Contains(out, "use-val") || !strings.Contains(out, "Convert to val") {
		t.Errorf("output missing first suggestion: %s", out)
	}
	if !strings.Contains(out, "informational") {
		t.Errorf("output missing informational tag for edit-less suggestion: %s", out)
	}
	if strings.Contains(out, "NoSuggestions") {
		t.Errorf("findings without suggestions should be omitted from --list: %s", out)
	}
}

func TestList_emptyReport(t *testing.T) {
	path := reportFile(t, output.JSONReport{})
	var stdout, stderr bytes.Buffer
	rc := run([]string{"--list", path}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no findings carry suggested fixes") {
		t.Errorf("expected empty-list notice, got: %s", stdout.String())
	}
}

func TestApply_modifiesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(src, []byte("var x = 1\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	report := output.JSONReport{
		Findings: []output.JSONFinding{{
			File: "a.kt", Line: 1, Column: 1, RuleSet: "style", Rule: "PreferVal",
			Message: "use val",
			SuggestedFixes: []output.JSONSuggestedFix{{
				ID: "use-val", Title: "Convert to val",
				Edits: []output.JSONSuggestedEdit{{StartLine: 1, EndLine: 1, Replacement: "val x = 1"}},
			}},
		}},
	}
	path := reportFile(t, report)

	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--finding", "PreferVal:a.kt:1:1",
		"--suggestion", "use-val",
		"--base", dir,
		path,
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	got, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read modified source: %v", err)
	}
	if string(got) != "val x = 1\n" {
		t.Errorf("file content: got %q want %q", got, "val x = 1\n")
	}
}

func TestApply_dryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.kt")
	original := []byte("var x = 1\n")
	if err := os.WriteFile(src, original, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	report := output.JSONReport{
		Findings: []output.JSONFinding{{
			File: "a.kt", Line: 1, Column: 1, Rule: "PreferVal",
			SuggestedFixes: []output.JSONSuggestedFix{{
				ID: "use-val", Title: "Convert to val",
				Edits: []output.JSONSuggestedEdit{{StartLine: 1, EndLine: 1, Replacement: "val x = 1"}},
			}},
		}},
	}
	path := reportFile(t, report)
	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--dry-run",
		"--finding", "PreferVal:a.kt:1:1",
		"--suggestion", "use-val",
		"--base", dir,
		path,
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	got, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("dry-run mutated file: got %q", got)
	}
	if !strings.Contains(stdout.String(), "dry-run") || !strings.Contains(stdout.String(), "val x = 1") {
		t.Errorf("dry-run output missing details: %s", stdout.String())
	}
}

func TestApply_staleFindingID(t *testing.T) {
	report := output.JSONReport{
		Findings: []output.JSONFinding{{
			File: "a.kt", Line: 1, Column: 1, Rule: "R",
			SuggestedFixes: []output.JSONSuggestedFix{{ID: "s1", Title: "t"}},
		}},
	}
	path := reportFile(t, report)
	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--finding", "R:a.kt:42:9",
		"--suggestion", "s1",
		path,
	}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("expected exit 1 for stale finding, got %d", rc)
	}
	if !strings.Contains(stderr.String(), "finding id") {
		t.Errorf("expected stale-finding error, got: %s", stderr.String())
	}
}

func TestApply_staleSuggestionID(t *testing.T) {
	report := output.JSONReport{
		Findings: []output.JSONFinding{{
			File: "a.kt", Line: 1, Column: 1, Rule: "R",
			SuggestedFixes: []output.JSONSuggestedFix{{ID: "s1", Title: "t"}},
		}},
	}
	path := reportFile(t, report)
	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--finding", "R:a.kt:1:1",
		"--suggestion", "nope",
		path,
	}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("expected exit 1 for stale suggestion, got %d", rc)
	}
	if !strings.Contains(stderr.String(), "suggestion id") {
		t.Errorf("expected stale-suggestion error, got: %s", stderr.String())
	}
}

func TestApply_informationalSuggestionRejected(t *testing.T) {
	report := output.JSONReport{
		Findings: []output.JSONFinding{{
			File: "a.kt", Line: 1, Column: 1, Rule: "R",
			SuggestedFixes: []output.JSONSuggestedFix{{
				ID: "explain", Title: "Read the docs",
				ApplicationToken: "help:docs",
			}},
		}},
	}
	path := reportFile(t, report)
	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--finding", "R:a.kt:1:1",
		"--suggestion", "explain",
		path,
	}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("expected exit 1 for non-machine-applicable suggestion, got %d", rc)
	}
	if !strings.Contains(stderr.String(), "not machine-applicable") {
		t.Errorf("expected machine-applicability error, got: %s", stderr.String())
	}
}

func TestApply_missingRequiredFlags(t *testing.T) {
	path := reportFile(t, output.JSONReport{})
	var stdout, stderr bytes.Buffer
	rc := run([]string{path}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("expected exit 1 when --finding/--suggestion are missing, got %d", rc)
	}
}

func TestApply_crossFileEdit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.kt")
	other := filepath.Join(dir, "b.kt")
	if err := os.WriteFile(src, []byte("var x = 1\n"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(other, []byte("var y = 2\n"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	report := output.JSONReport{
		Findings: []output.JSONFinding{{
			File: "a.kt", Line: 1, Column: 1, Rule: "R",
			SuggestedFixes: []output.JSONSuggestedFix{{
				ID: "cross", Title: "edits across files",
				Edits: []output.JSONSuggestedEdit{
					{StartLine: 1, EndLine: 1, Replacement: "val x = 1"},
					{TargetFile: "b.kt", StartLine: 1, EndLine: 1, Replacement: "val y = 2"},
				},
			}},
		}},
	}
	path := reportFile(t, report)
	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--finding", "R:a.kt:1:1",
		"--suggestion", "cross",
		"--base", dir,
		path,
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	gotA, _ := os.ReadFile(src)
	gotB, _ := os.ReadFile(other)
	if string(gotA) != "val x = 1\n" {
		t.Errorf("a.kt: got %q", gotA)
	}
	if string(gotB) != "val y = 2\n" {
		t.Errorf("b.kt: got %q", gotB)
	}
}

// TestFix_doesNotApplySuggestedEdits guards the mutual exclusion between
// --fix and suggested fixes. A finding with no autofix (Fix=nil) but a
// suggested fix carrying edits must not be touched by the
// fixer.ApplyAllFixesColumns path that --fix drives.
func TestFix_doesNotApplySuggestedEdits(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.kt")
	original := []byte("var x = 1\n")
	if err := os.WriteFile(src, original, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	findings := []scanner.Finding{{
		File:    src,
		Line:    1,
		Col:     1,
		RuleSet: "style",
		Rule:    "PreferVal",
		Message: "use val",
		SuggestedFixes: []scanner.SuggestedFix{{
			ID: "use-val", Title: "Convert to val",
			Edits: []scanner.SuggestedEdit{{StartLine: 1, EndLine: 1, Replacement: "val x = 1"}},
		}},
		// No Fix slot: suggested fix is not an autofix.
	}}
	cols := scanner.CollectFindings(findings)
	applied, modified, errs := fixer.ApplyAllFixesColumns(t.Context(), &cols, "")
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if applied != 0 || modified != 0 {
		t.Fatalf("--fix should not apply suggested fixes: applied=%d modified=%d", applied, modified)
	}
	got, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("source mutated by --fix path despite no autofix: got %q", got)
	}
}
