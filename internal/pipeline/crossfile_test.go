package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestCrossFilePhase_Name(t *testing.T) {
	if (CrossFilePhase{}).Name() != "crossfile" {
		t.Fatalf("Name = %q", (CrossFilePhase{}).Name())
	}
}

// parsedKotlinFile is a small helper: writes content to a temp file,
// parses it through scanner.ParseFile, and installs a SuppressionFilter
// the way ParsePhase would. Used by multiple tests below.
func parsedKotlinFile(t *testing.T, content string) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "file.kt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	f.Suppression = scanner.BuildSuppressionFilter(f, nil, nil, "")
	f.SuppressionIdx = f.Suppression.Annotations()
	return f
}

func TestCrossFilePhase_NoRules_PassesThrough(t *testing.T) {
	file := parsedKotlinFile(t, "class X\n")
	in := DispatchResult{
		IndexResult: IndexResult{
			ParseResult: ParseResult{
				KotlinFiles: []*scanner.File{file},
				ActiveRules: nil,
			},
		},
		Findings: scanner.CollectFindings(nil),
	}
	out, err := CrossFilePhase{}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Findings.Len() != 0 {
		t.Errorf("Findings.Len = %d, want 0", out.Findings.Len())
	}
}

func TestCrossFilePhase_PerFileFindingsAreCarriedForward(t *testing.T) {
	file := parsedKotlinFile(t, "class X\n")
	perFileFinding := scanner.Finding{
		File: file.Path, Line: 1, Rule: "PerFileRule", RuleSet: "test", Message: "m",
	}
	in := DispatchResult{
		IndexResult: IndexResult{
			ParseResult: ParseResult{
				KotlinFiles: []*scanner.File{file},
				ActiveRules: nil,
			},
		},
		Findings: scanner.CollectFindings([]scanner.Finding{perFileFinding}),
	}
	out, err := CrossFilePhase{}.Run(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Findings.Len() != 1 {
		t.Errorf("Findings.Len = %d, want 1 (per-file finding must carry forward)", out.Findings.Len())
	}
}

// TestCrossFilePhase_SuppressionAppliedToCrossFileFinding is the
// acceptance regression for the PhasePipeline roadmap (criterion #3):
// findings emitted by cross-file rules MUST be filtered through the same
// @Suppress index as per-file findings.
func TestCrossFilePhase_SuppressionAppliedToCrossFileFinding(t *testing.T) {
	// Line 1 carries @Suppress("DeadSymbol") so any finding whose Rule
	// is "DeadSymbol" on line 1+2 (the annotated declaration's scope)
	// should be filtered out.
	src := `@Suppress("DeadSymbol")
class UnusedClass
`
	file := parsedKotlinFile(t, src)

	// We validate the suppression behaviour via ApplySuppression directly;
	// the cross-file rule invocation path is exercised by existing
	// integration smoke against the playground.
	kept := ApplySuppression(
		[]scanner.Finding{
			{File: file.Path, Line: 2, Rule: "DeadSymbol", RuleSet: "test", Message: "unused"},
			{File: file.Path, Line: 2, Rule: "OtherRule", RuleSet: "test", Message: "keep"},
		},
		[]*scanner.File{file},
	)
	if len(kept) != 1 {
		t.Fatalf("ApplySuppression kept %d findings, want 1; got %+v", len(kept), kept)
	}
	if kept[0].Rule != "OtherRule" {
		t.Errorf("kept rule = %q, want OtherRule (DeadSymbol should be suppressed)", kept[0].Rule)
	}

}

func TestCrossFilePhase_SuppressionPassesThrough_WhenNoIndex(t *testing.T) {
	// Files without a SuppressionIdx (e.g. produced outside ParsePhase)
	// must not have their findings dropped.
	dir := t.TempDir()
	p := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(p, []byte("class A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately do NOT set f.SuppressionIdx.

	kept := ApplySuppression(
		[]scanner.Finding{{File: f.Path, Line: 1, Rule: "X", RuleSet: "test", Message: "m"}},
		[]*scanner.File{f},
	)
	if len(kept) != 1 {
		t.Errorf("len(kept) = %d, want 1 (no index = no suppression)", len(kept))
	}
}

func TestCrossFilePhase_SuppressionPassesThrough_WhenFileUnknown(t *testing.T) {
	// Finding refers to a file not in the parsed set (Java, XML, etc.).
	kept := ApplySuppression(
		[]scanner.Finding{{File: "/nowhere/foo.java", Line: 1, Rule: "X", RuleSet: "test"}},
		nil,
	)
	if len(kept) != 1 {
		t.Errorf("len(kept) = %d, want 1 (unknown file = pass through)", len(kept))
	}
}

func TestCrossFilePhase_ContextCancel(t *testing.T) {
	in := DispatchResult{
		IndexResult: IndexResult{
			ParseResult: ParseResult{KotlinFiles: nil, ActiveRules: nil},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runPhase[DispatchResult, CrossFileResult](ctx, CrossFilePhase{}, in)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	var pe *PhaseError
	if !errors.As(err, &pe) || pe.Phase != "crossfile" {
		t.Fatalf("want PhaseError phase=crossfile, got %v", err)
	}
}

// TestCrossFilePhase_InlineIgnoreSuppressesCrossFileFinding proves
// that the unified SuppressionFilter applies to cross-file findings
// for inline `// krit:ignore[Rule]` comments, not just @Suppress.
// Closes SuppressionMiddleware acceptance criterion #3 for the
// inline-comment source.
func TestCrossFilePhase_InlineIgnoreSuppressesCrossFileFinding(t *testing.T) {
	src := "class UnusedClass // krit:ignore[DeadSymbol]\nclass OtherClass\n"
	file := parsedKotlinFile(t, src)

	kept := ApplySuppression(
		[]scanner.Finding{
			{File: file.Path, Line: 1, Rule: "DeadSymbol", RuleSet: "test", Message: "unused"},
			{File: file.Path, Line: 1, Rule: "OtherRule", RuleSet: "test", Message: "keep"},
			{File: file.Path, Line: 2, Rule: "DeadSymbol", RuleSet: "test", Message: "keep"},
		},
		[]*scanner.File{file},
	)
	if len(kept) != 2 {
		t.Fatalf("ApplySuppression kept %d, want 2; got %+v", len(kept), kept)
	}
	for _, f := range kept {
		if f.Rule == "DeadSymbol" && f.Line == 1 {
			t.Errorf("inline ignore should have suppressed DeadSymbol on line 1, got %+v", f)
		}
	}
}

// TestCrossFilePhase_ExcludeGlobSuppressesCrossFileFinding proves that
// a config-level rule exclude glob matching the finding's file causes
// the cross-file finding to drop, closing the rule-exclude path through
// the same SuppressionFilter.
func TestCrossFilePhase_ExcludeGlobSuppressesCrossFileFinding(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "SomethingTest.kt")
	if err := os.WriteFile(p, []byte("class T\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	f.Suppression = scanner.BuildSuppressionFilter(f, nil, map[string][]string{
		"DeadSymbol": {"**/*Test.kt"},
	}, "")
	f.SuppressionIdx = f.Suppression.Annotations()

	kept := ApplySuppression(
		[]scanner.Finding{
			{File: f.Path, Line: 1, Rule: "DeadSymbol", RuleSet: "test"},
			{File: f.Path, Line: 1, Rule: "OtherRule", RuleSet: "test"},
		},
		[]*scanner.File{f},
	)
	if len(kept) != 1 {
		t.Fatalf("kept %d, want 1; got %+v", len(kept), kept)
	}
	if kept[0].Rule != "OtherRule" {
		t.Errorf("kept rule = %q, want OtherRule (DeadSymbol file-excluded)", kept[0].Rule)
	}
}

