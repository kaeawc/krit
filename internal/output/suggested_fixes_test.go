package output

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestSuggestedFixes_JSONShape covers the fixture-rule acceptance criterion:
// a rule emits one finding with two suggested fixes, and both appear in the
// JSON output in rule-defined order. Also verifies that the autofix metadata
// (`fixable` / `fixLevel`) is independent of suggestions — a finding can
// carry suggestions without being autofixable.
func TestSuggestedFixes_JSONShape(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:     "a.kt",
			Line:     5,
			Col:      3,
			Severity: "warning",
			RuleSet:  "style",
			Rule:     "FixtureSuggester",
			Message:  "two suggestions please",
			SuggestedFixes: []scanner.SuggestedFix{
				{
					ID:    "use-val",
					Title: "Convert to val",
					Edits: []scanner.SuggestedEdit{
						{StartLine: 5, EndLine: 5, Replacement: "val x = 1"},
					},
				},
				{
					ID:               "explain",
					Title:            "Explain the warning",
					Detail:           "var becomes val when the binding is read-only.",
					ApplicationToken: "help:val-vs-var",
				},
			},
		},
	}

	report := runJSONFormatter(t, findings)
	if len(report.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(report.Findings))
	}
	got := report.Findings[0]

	// Suggestions are orthogonal to the autofix slot.
	if got.Fixable {
		t.Errorf("suggestions must not make a finding fixable: got fixable=true")
	}
	if got.FixLevel != "" {
		t.Errorf("suggestions must not populate fixLevel: got %q", got.FixLevel)
	}

	if len(got.SuggestedFixes) != 2 {
		t.Fatalf("want 2 suggested fixes, got %d", len(got.SuggestedFixes))
	}

	// Rule-defined order is preserved.
	if got.SuggestedFixes[0].ID != "use-val" {
		t.Errorf("suggestion 0 id: got %q want %q", got.SuggestedFixes[0].ID, "use-val")
	}
	if got.SuggestedFixes[1].ID != "explain" {
		t.Errorf("suggestion 1 id: got %q want %q", got.SuggestedFixes[1].ID, "explain")
	}
	if got.SuggestedFixes[0].Title != "Convert to val" {
		t.Errorf("suggestion 0 title: got %q", got.SuggestedFixes[0].Title)
	}
	if len(got.SuggestedFixes[0].Edits) != 1 {
		t.Fatalf("suggestion 0 edits: got %d", len(got.SuggestedFixes[0].Edits))
	}
	if got.SuggestedFixes[0].Edits[0].Replacement != "val x = 1" {
		t.Errorf("edit replacement: got %q", got.SuggestedFixes[0].Edits[0].Replacement)
	}
	if got.SuggestedFixes[1].Detail != "var becomes val when the binding is read-only." {
		t.Errorf("suggestion 1 detail: got %q", got.SuggestedFixes[1].Detail)
	}
	if got.SuggestedFixes[1].ApplicationToken != "help:val-vs-var" {
		t.Errorf("suggestion 1 application token: got %q", got.SuggestedFixes[1].ApplicationToken)
	}
	if len(got.SuggestedFixes[1].Edits) != 0 {
		t.Errorf("suggestion 1 should be non-machine-applicable: got %d edits",
			len(got.SuggestedFixes[1].Edits))
	}
}

// TestSuggestedFixes_DeterministicSerialization formats the same finding set
// twice and verifies the byte output is identical. This is the cold vs. warm
// determinism guarantee — the merge / sort / format pipeline must not depend
// on Go map iteration order or pool growth ordering for findings with
// multiple suggestions.
func TestSuggestedFixes_DeterministicSerialization(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:    "b.kt",
			Line:    1,
			Col:     1,
			RuleSet: "style",
			Rule:    "R2",
			Message: "two",
			SuggestedFixes: []scanner.SuggestedFix{
				{ID: "b-1", Title: "B1"},
				{ID: "b-2", Title: "B2"},
				{ID: "b-3", Title: "B3"},
			},
			Severity: "warning",
		},
		{
			File:    "a.kt",
			Line:    1,
			Col:     1,
			RuleSet: "style",
			Rule:    "R1",
			Message: "one",
			SuggestedFixes: []scanner.SuggestedFix{
				{ID: "a-1", Title: "A1", Edits: []scanner.SuggestedEdit{
					{StartLine: 1, EndLine: 1, Replacement: "x"},
				}},
				{ID: "a-2", Title: "A2"},
			},
			Severity: "warning",
		},
	}

	first := runJSONFormatterBytes(t, findings)
	second := runJSONFormatterBytes(t, findings)
	if !bytes.Equal(first, second) {
		t.Fatalf("non-deterministic JSON output:\n--- first ---\n%s\n--- second ---\n%s",
			first, second)
	}

	// Cross-collector merge must preserve per-finding suggestion ordering.
	mergedCols := mergedCollectorColumns(findings)
	formatA := formatColumns(t, mergedCols)
	formatB := formatColumns(t, mergedCols)
	if !bytes.Equal(formatA, formatB) {
		t.Fatalf("non-deterministic JSON output from merged collector")
	}

	var report JSONReport
	if err := json.Unmarshal(first, &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("want 2 findings, got %d", len(report.Findings))
	}
	// a.kt sorts before b.kt — verify suggestion order inside each finding.
	if report.Findings[0].File != "a.kt" {
		t.Fatalf("expected a.kt first, got %q", report.Findings[0].File)
	}
	if ids := suggestionIDs(report.Findings[0]); !slices.Equal(ids, []string{"a-1", "a-2"}) {
		t.Errorf("a.kt suggestion order: %v", ids)
	}
	if ids := suggestionIDs(report.Findings[1]); !slices.Equal(ids, []string{"b-1", "b-2", "b-3"}) {
		t.Errorf("b.kt suggestion order: %v", ids)
	}
}

// TestSuggestedFixes_ColdWarmRoundTrip exercises the FindingColumns cache
// codec: serialize to JSON and back, then format from the rehydrated columns.
// The resulting output must equal the cold-run output byte-for-byte.
func TestSuggestedFixes_ColdWarmRoundTrip(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:     "a.kt",
			Line:     2,
			Col:      4,
			Severity: "warning",
			RuleSet:  "style",
			Rule:     "Cold",
			Message:  "cached",
			SuggestedFixes: []scanner.SuggestedFix{
				{ID: "s1", Title: "First"},
				{ID: "s2", Title: "Second", Edits: []scanner.SuggestedEdit{
					{StartByte: 10, EndByte: 12, ByteMode: true, Replacement: "ok"},
				}},
			},
		},
	}

	cold := scanner.CollectFindings(findings)
	encoded, err := json.Marshal(&cold)
	if err != nil {
		t.Fatalf("marshal cold columns: %v", err)
	}
	var warm scanner.FindingColumns
	if err := json.Unmarshal(encoded, &warm); err != nil {
		t.Fatalf("unmarshal warm columns: %v", err)
	}

	coldJSON := formatColumns(t, &cold)
	warmJSON := formatColumns(t, &warm)
	if !bytes.Equal(coldJSON, warmJSON) {
		t.Fatalf("cold vs warm JSON drift:\ncold: %s\nwarm: %s", coldJSON, warmJSON)
	}

	got := warm.SuggestedFixesAt(0)
	if len(got) != 2 {
		t.Fatalf("warm row should carry 2 suggestions, got %d", len(got))
	}
	if got[0].ID != "s1" || got[1].ID != "s2" {
		t.Errorf("warm suggestion order: %q,%q want s1,s2", got[0].ID, got[1].ID)
	}
	if !got[1].Edits[0].ByteMode {
		t.Errorf("warm suggestion 2 edit ByteMode lost")
	}
}

// TestAppendFindingJSON_MatchesJSONMarshal_WithSuggestedFixes pins the
// byte-identical contract for the hand-rolled encoder when a finding has
// suggested fixes attached. The contract is broader than this case but the
// hand-rolled path uses json.Marshal for the suggestions slice; this guards
// against accidental field-order drift in JSONFinding.
func TestAppendFindingJSON_MatchesJSONMarshal_WithSuggestedFixes(t *testing.T) {
	f := JSONFinding{
		File: "A.kt", Line: 1, Column: 1,
		RuleSet: "style", Rule: "R", Severity: "warning",
		Message: "m", Confidence: 0.75,
		SuggestedFixes: []JSONSuggestedFix{
			{
				ID:    "s1",
				Title: "Title 1",
				Edits: []JSONSuggestedEdit{
					{StartLine: 1, EndLine: 2, Replacement: "x"},
				},
			},
			{
				ID:               "s2",
				Title:            "Title 2",
				Detail:           "detail text",
				ApplicationToken: "tok",
			},
		},
	}
	want, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	got := appendFindingJSON(nil, f)
	if !bytes.Equal(got, want) {
		t.Fatalf("byte mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func runJSONFormatter(t *testing.T, findings []scanner.Finding) JSONReport {
	t.Helper()
	body := runJSONFormatterBytes(t, findings)
	var report JSONReport
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return report
}

func runJSONFormatterBytes(t *testing.T, findings []scanner.Finding) []byte {
	t.Helper()
	cols := scanner.CollectFindings(findings)
	return formatColumns(t, &cols)
}

func formatColumns(t *testing.T, cols *scanner.FindingColumns) []byte {
	t.Helper()
	var buf bytes.Buffer
	err := FormatJSONColumns(&buf, cols, "test-version", 1, 1, time.Unix(0, 0),
		nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("FormatJSONColumns: %v", err)
	}
	return buf.Bytes()
}

func mergedCollectorColumns(findings []scanner.Finding) *scanner.FindingColumns {
	workerA := scanner.NewFindingCollector(0)
	workerB := scanner.NewFindingCollector(0)
	for i, f := range findings {
		if i%2 == 0 {
			workerA.Append(f)
		} else {
			workerB.Append(f)
		}
	}
	merged := scanner.MergeCollectors(nil, workerA, workerB)
	return merged.Columns()
}

func suggestionIDs(f JSONFinding) []string {
	out := make([]string, len(f.SuggestedFixes))
	for i, s := range f.SuggestedFixes {
		out[i] = s.ID
	}
	return out
}
