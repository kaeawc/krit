package lsp

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestFindingsToCodeActionsAddsPreviewForIdiomaticFix(t *testing.T) {
	uri := "file:///tmp/test/Test.kt"
	content := "fun requireValue(value: String?) {\n    if (value == null) throw IllegalArgumentException(\"missing\")\n}\n"
	oldText := "if (value == null) throw IllegalArgumentException(\"missing\")"
	startByte := strings.Index(content, oldText)
	if startByte < 0 {
		t.Fatal("expected to find original text in content")
	}

	findings := []scanner.Finding{
		{
			File:     "/tmp/test/Test.kt",
			Line:     2,
			Col:      5,
			RuleSet:  "style",
			Rule:     "UseRequireNotNull",
			Severity: "warning",
			Message:  "use requireNotNull",
			Fix: &scanner.Fix{
				StartByte:   startByte,
				EndByte:     startByte + len(oldText),
				Replacement: "requireNotNull(value) { \"missing\" }",
				ByteMode:    true,
			},
		},
	}

	actions := findingsToCodeActions(uri, content, findings)
	if len(actions) != 1 {
		t.Fatalf("expected 1 code action, got %d", len(actions))
	}

	if actions[0].Data == nil || actions[0].Data.Preview == nil {
		t.Fatal("expected quick-fix preview data for idiomatic fix")
	}

	preview := actions[0].Data.Preview
	if preview.FixLevel != "idiomatic" {
		t.Fatalf("fixLevel: got %q, want %q", preview.FixLevel, "idiomatic")
	}
	if !strings.Contains(preview.Diff, "@@") {
		t.Fatalf("expected unified diff header, got %q", preview.Diff)
	}
	if !strings.Contains(preview.Diff, "-    if (value == null) throw IllegalArgumentException(\"missing\")") {
		t.Fatalf("expected removed line in diff, got %q", preview.Diff)
	}
	if !strings.Contains(preview.Diff, "+    requireNotNull(value) { \"missing\" }") {
		t.Fatalf("expected added line in diff, got %q", preview.Diff)
	}
}

func TestFindingsToCodeActionsSkipsPreviewForCosmeticFix(t *testing.T) {
	uri := "file:///tmp/test/Test.kt"
	content := "fun main() {   \n}\n"

	findings := []scanner.Finding{
		{
			File:     "/tmp/test/Test.kt",
			Line:     1,
			Col:      13,
			RuleSet:  "style",
			Rule:     "TrailingWhitespace",
			Severity: "warning",
			Message:  "remove trailing whitespace",
			Fix: &scanner.Fix{
				StartByte:   12,
				EndByte:     15,
				Replacement: "",
				ByteMode:    true,
			},
		},
	}

	actions := findingsToCodeActions(uri, content, findings)
	if len(actions) != 1 {
		t.Fatalf("expected 1 code action, got %d", len(actions))
	}
	if actions[0].Data != nil {
		t.Fatalf("expected no preview data for cosmetic fix, got %+v", actions[0].Data)
	}
}
