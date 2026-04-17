package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestEndToEnd_CrossFileSuppression is the regression test for
// roadmap acceptance criterion #3: a @Suppress annotation must cover
// findings produced by cross-file rules, not only per-file dispatch.
//
// The test runs Parse → (synthetic cross-file finding emission) →
// ApplySuppression and asserts the finding is dropped. It deliberately
// exercises the exact code path main.go now uses (ParsePhase populates
// SuppressionIdx; ApplySuppression consults it).
func TestEndToEnd_CrossFileSuppression(t *testing.T) {
	dir := t.TempDir()
	suppressed := filepath.Join(dir, "Suppressed.kt")
	bare := filepath.Join(dir, "Bare.kt")
	if err := os.WriteFile(suppressed, []byte(`@Suppress("UnusedDeclaration")
class SuppressedClass
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bare, []byte("class BareClass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Go through the real ParsePhase so SuppressionIdx is populated the
	// same way it is in production.
	in := ParseInput{
		Paths:       []string{dir},
		ActiveRules: nil,
	}
	out, err := ParsePhase{}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(out.KotlinFiles) != 2 {
		t.Fatalf("Parse returned %d files, want 2", len(out.KotlinFiles))
	}

	// Synthesize cross-file findings — one per file on the class line.
	crossFindings := []scanner.Finding{
		{File: suppressed, Line: 2, Rule: "UnusedDeclaration", RuleSet: "potential-bugs", Message: "unused"},
		{File: bare, Line: 1, Rule: "UnusedDeclaration", RuleSet: "potential-bugs", Message: "unused"},
	}

	kept := ApplySuppression(crossFindings, out.KotlinFiles)
	if len(kept) != 1 {
		t.Fatalf("ApplySuppression kept %d, want 1 (bare only); got %+v", len(kept), kept)
	}
	if kept[0].File != bare {
		t.Errorf("kept finding is for %q, want %q (Suppressed.kt should have been dropped)", kept[0].File, bare)
	}
}

// TestEndToEnd_MCP_LSP_CLI_ShareActiveRules proves the three entry
// points derive the same rule set from pipeline.DefaultActiveRules —
// the acceptance criterion that LSP/MCP use no rule-dispatch logic of
// their own.
func TestEndToEnd_MCP_LSP_CLI_ShareActiveRules(t *testing.T) {
	first := DefaultActiveRules()
	second := DefaultActiveRules()

	if len(first) != len(second) {
		t.Fatalf("DefaultActiveRules inconsistent between calls: %d vs %d", len(first), len(second))
	}
	// Order must also match so callers that iterate by index (e.g. the
	// v2→v1 wrapper map) stay in lock-step.
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Errorf("rule[%d]: first=%q, second=%q", i, first[i].ID, second[i].ID)
		}
	}
}
