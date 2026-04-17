package mcp

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestFilterFindingColumns_PreservesFixPayloads(t *testing.T) {
	columns := scanner.CollectFindings([]scanner.Finding{
		{
			File:     "src/Keep.kt",
			Line:     3,
			Col:      1,
			RuleSet:  "style",
			Rule:     "KeepRule",
			Severity: "warning",
			Message:  "keep me",
			Fix: &scanner.Fix{
				StartLine:   3,
				EndLine:     3,
				Replacement: "fixed()",
			},
			BinaryFix: &scanner.BinaryFix{
				Type:    scanner.BinaryFixCreateFile,
				Content: []byte("payload"),
			},
			Confidence: 0.91,
		},
		{
			File:     "src/Drop.kt",
			Line:     8,
			Col:      2,
			RuleSet:  "style",
			Rule:     "DropRule",
			Severity: "error",
			Message:  "drop me",
		},
	})

	filtered := filterFindingColumns(&columns, func(columns *scanner.FindingColumns, row int) bool {
		return columns.RuleAt(row) == "KeepRule"
	})

	want := []scanner.Finding{{
		File:     "src/Keep.kt",
		Line:     3,
		Col:      1,
		RuleSet:  "style",
		Rule:     "KeepRule",
		Severity: "warning",
		Message:  "keep me",
		Fix: &scanner.Fix{
			StartLine:   3,
			EndLine:     3,
			Replacement: "fixed()",
		},
		BinaryFix: &scanner.BinaryFix{
			Type:    scanner.BinaryFixCreateFile,
			Content: []byte("payload"),
		},
		Confidence: 0.91,
	}}

	if got := filtered.Findings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered findings mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestFindingsToResultColumns_MatchesLegacySlicePath(t *testing.T) {
	findings := []scanner.Finding{
		{
			File:     "src/A.kt",
			Line:     1,
			Col:      2,
			RuleSet:  "style",
			Rule:     "Alpha",
			Severity: "warning",
			Message:  "alpha",
			Fix: &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "fixed()",
			},
		},
		{
			File:     "src/B.kt",
			Line:     4,
			Col:      5,
			RuleSet:  "bugs",
			Rule:     "Beta",
			Severity: "error",
			Message:  "beta",
		},
	}
	columns := scanner.CollectFindings(findings)

	want := findingsToResult(findings)
	got := findingsToResultColumns(&columns)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}

	var decoded []map[string]interface{}
	if err := json.Unmarshal([]byte(got.Content[0].Text), &decoded); err != nil {
		t.Fatalf("result text should be valid JSON: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 result rows, got %d", len(decoded))
	}
}
