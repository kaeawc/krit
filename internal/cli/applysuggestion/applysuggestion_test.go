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

// TestApply_overlappingEditsReportPostDedupCount is the regression for
// the apply-suggestion reporting bug: when two edits in the same
// suggestion overlap, fixer.deduplicateFixesReverse drops one, but the
// CLI used to report the full submitted count. The success line must
// report the post-dedup applied count, and the dropped edit must be
// surfaced as a warning with a clear reason on stderr.
func TestApply_overlappingEditsReportPostDedupCount(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(src, []byte("abcdefghij"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	// Two byte-mode edits that overlap at bytes 2-8 — dedup keeps the
	// longer span (rule emits two via the same suggestion id).
	report := output.JSONReport{
		Findings: []output.JSONFinding{{
			File: "a.kt", Line: 1, Column: 1, RuleSet: "demo", Rule: "OverlapRule",
			Message: "overlap",
			SuggestedFixes: []output.JSONSuggestedFix{{
				ID: "overlap", Title: "overlapping edits",
				Edits: []output.JSONSuggestedEdit{
					{StartByte: 2, EndByte: 5, ByteMode: true, Replacement: "Z"},
					{StartByte: 2, EndByte: 8, ByteMode: true, Replacement: "A"},
				},
			}},
		}},
	}
	path := reportFile(t, report)
	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--finding", "OverlapRule:a.kt:1:1",
		"--suggestion", "overlap",
		"--base", dir,
		path,
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "1 edit(s) across") {
		t.Errorf("expected post-dedup applied count of 1, got: %s", out)
	}
	if strings.Contains(out, "2 edit(s) across") {
		t.Errorf("output still reports inflated submitted count: %s", out)
	}
	if !strings.Contains(out, "1 edit(s) dropped") {
		t.Errorf("expected dropped summary on stdout, got: %s", out)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "dropped edit") || !strings.Contains(errOut, "because") {
		t.Errorf("expected dropped-edit warning with reason on stderr, got: %s", errOut)
	}
	got, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read modified source: %v", err)
	}
	if string(got) != "abAij" {
		t.Errorf("file content: got %q want %q", got, "abAij")
	}
}

// TestApply_droppedReasonIsSurfaced asserts the canonical overlap
// reason string flows through to user-visible output, so the user can
// understand why an edit was skipped without re-reading the fixer
// source.
func TestApply_droppedReasonIsSurfaced(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(src, []byte("abcdefghij"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	report := output.JSONReport{
		Findings: []output.JSONFinding{{
			File: "a.kt", Line: 1, Column: 1, Rule: "OverlapRule",
			SuggestedFixes: []output.JSONSuggestedFix{{
				ID: "overlap",
				Edits: []output.JSONSuggestedEdit{
					{StartByte: 2, EndByte: 5, ByteMode: true, Replacement: "Z"},
					{StartByte: 2, EndByte: 8, ByteMode: true, Replacement: "A"},
				},
			}},
		}},
	}
	path := reportFile(t, report)
	var stdout, stderr bytes.Buffer
	rc := run([]string{
		"--finding", "OverlapRule:a.kt:1:1",
		"--suggestion", "overlap",
		"--base", dir,
		path,
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	errOut := stderr.String()
	// The reason text comes from internal/fixer; require a substring
	// from the canonical message so a typo there fails the test.
	if !strings.Contains(errOut, "overlaps with") {
		t.Errorf("expected canonical overlap reason in warning, got: %s", errOut)
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
