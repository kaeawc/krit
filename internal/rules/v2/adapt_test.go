package v2

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestAdaptFlatDispatch(t *testing.T) {
	called := false
	r := AdaptFlatDispatch(
		"TestFlatRule", "style", "test flat dispatch", SeverityWarning,
		[]string{"call_expression"},
		func(idx uint32, file *scanner.File) []scanner.Finding {
			called = true
			return []scanner.Finding{{
				File: file.Path, Line: int(idx) + 1, Col: 1,
				Rule: "TestFlatRule", RuleSet: "style", Severity: "warning",
				Message: "test finding",
			}}
		},
		AdaptWithConfidence(0.95),
		AdaptWithFix(FixCosmetic),
	)

	if r.ID != "TestFlatRule" {
		t.Errorf("ID = %q, want TestFlatRule", r.ID)
	}
	if !r.Needs.IsPerFile() {
		t.Error("expected per-file rule")
	}
	if r.Needs.Has(NeedsLinePass) {
		t.Error("flat dispatch should not have NeedsLinePass")
	}
	if r.Fix != FixCosmetic {
		t.Errorf("Fix = %v, want FixCosmetic", r.Fix)
	}
	if r.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", r.Confidence)
	}

	file := &scanner.File{Path: "test.kt"}
	ctx := &Context{File: file, Idx: 5}
	r.Check(ctx)

	if !called {
		t.Error("check function was not called")
	}
	if len(ctx.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(ctx.Findings))
	}
	if ctx.Findings[0].Line != 6 {
		t.Errorf("finding line = %d, want 6", ctx.Findings[0].Line)
	}
}

func TestAdaptLine(t *testing.T) {
	called := false
	r := AdaptLine(
		"TestLineRule", "naming", "test line rule", SeverityInfo,
		func(file *scanner.File) []scanner.Finding {
			called = true
			return []scanner.Finding{{
				File: file.Path, Line: 1, Col: 1,
				Rule: "TestLineRule", Message: "line finding",
			}}
		},
	)

	if !r.Needs.Has(NeedsLinePass) {
		t.Error("expected NeedsLinePass capability")
	}
	if !r.Needs.IsPerFile() {
		t.Error("line rule should be per-file")
	}

	file := &scanner.File{Path: "test.kt"}
	ctx := &Context{File: file}
	r.Check(ctx)

	if !called {
		t.Error("check function was not called")
	}
	if len(ctx.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(ctx.Findings))
	}
}

func TestAdaptCrossFile(t *testing.T) {
	called := false
	r := AdaptCrossFile(
		"TestCrossFile", "complexity", "test cross-file", SeverityWarning,
		func(index *scanner.CodeIndex) []scanner.Finding {
			called = true
			return []scanner.Finding{{
				File: "a.kt", Line: 1, Message: "cross-file finding",
			}}
		},
	)

	if !r.Needs.Has(NeedsCrossFile) {
		t.Error("expected NeedsCrossFile capability")
	}
	if r.Needs.IsPerFile() {
		t.Error("cross-file rule should not be per-file")
	}

	ctx := &Context{CodeIndex: &scanner.CodeIndex{}}
	r.Check(ctx)

	if !called {
		t.Error("check function was not called")
	}
	if len(ctx.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(ctx.Findings))
	}
}

func TestAdaptParsedFiles(t *testing.T) {
	called := false
	r := AdaptParsedFiles(
		"TestParsedFiles", "style", "test parsed files", SeverityWarning,
		func(files []*scanner.File) []scanner.Finding {
			called = true
			return []scanner.Finding{{
				File: "a.kt", Line: 1, Message: "parsed files finding",
			}}
		},
	)

	if !r.Needs.Has(NeedsParsedFiles) {
		t.Error("expected NeedsParsedFiles capability")
	}

	ctx := &Context{ParsedFiles: []*scanner.File{{Path: "a.kt"}}}
	r.Check(ctx)

	if !called {
		t.Error("check function was not called")
	}
}

func TestAdaptWithNeeds(t *testing.T) {
	r := AdaptFlatDispatch(
		"NeedsResolver", "style", "needs resolver", SeverityWarning,
		[]string{"call_expression"},
		func(idx uint32, file *scanner.File) []scanner.Finding { return nil },
		AdaptWithNeeds(NeedsResolver),
	)

	if !r.Needs.Has(NeedsResolver) {
		t.Error("expected NeedsResolver capability")
	}
}

func TestV1FlatDispatchWrapper(t *testing.T) {
	r := FakeRule("V1Compat",
		WithNodeTypes("call_expression"),
		WithSeverity(SeverityWarning),
		WithConfidence(0.85),
		WithFix(FixIdiomatic),
		WithCheck(func(ctx *Context) {
			ctx.EmitAt(10, 5, "found issue")
		}),
	)
	r.Category = "style"

	w := &V1FlatDispatch{R: r}

	if w.Name() != "V1Compat" {
		t.Errorf("Name() = %q, want V1Compat", w.Name())
	}
	if w.RuleSet() != "style" {
		t.Errorf("RuleSet() = %q, want style", w.RuleSet())
	}
	if w.Severity() != "warning" {
		t.Errorf("Severity() = %q, want warning", w.Severity())
	}
	if !w.IsFixable() {
		t.Error("expected IsFixable() = true")
	}
	if w.Confidence() != 0.85 {
		t.Errorf("Confidence() = %f, want 0.85", w.Confidence())
	}

	// Check returns nil (stub)
	if got := w.Check(nil); got != nil {
		t.Errorf("Check() = %v, want nil", got)
	}

	file := &scanner.File{Path: "test.kt"}
	findings := w.CheckFlatNode(3, file)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Rule != "V1Compat" {
		t.Errorf("finding.Rule = %q, want V1Compat", f.Rule)
	}
	if f.RuleSet != "style" {
		t.Errorf("finding.RuleSet = %q, want style", f.RuleSet)
	}
	if f.Severity != "warning" {
		t.Errorf("finding.Severity = %q, want warning", f.Severity)
	}
	if f.File != "test.kt" {
		t.Errorf("finding.File = %q, want test.kt", f.File)
	}
}

func TestV1LineWrapper(t *testing.T) {
	r := FakeRule("LineCompat",
		WithNeeds(NeedsLinePass),
		WithCheck(func(ctx *Context) {
			ctx.EmitAt(1, 1, "line issue")
		}),
	)
	r.Category = "naming"

	w := &V1Line{R: r}

	if w.Name() != "LineCompat" {
		t.Errorf("Name() = %q, want LineCompat", w.Name())
	}

	file := &scanner.File{Path: "test.kt"}
	findings := w.CheckLines(file)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "LineCompat" {
		t.Errorf("finding.Rule = %q, want LineCompat", findings[0].Rule)
	}
}

func TestV1CrossFileWrapper(t *testing.T) {
	r := FakeRule("CrossCompat",
		WithNeeds(NeedsCrossFile),
		WithCheck(func(ctx *Context) {
			ctx.Emit(scanner.Finding{
				File: "a.kt", Line: 1, Message: "cross issue",
			})
		}),
	)
	r.Category = "complexity"

	w := &V1CrossFile{R: r}

	findings := w.CheckCrossFile(&scanner.CodeIndex{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "CrossCompat" {
		t.Errorf("finding.Rule = %q, want CrossCompat", findings[0].Rule)
	}
}

func TestToV1_Routing(t *testing.T) {
	flatRule := FakeRule("flat", WithNodeTypes("call_expression"))
	lineRule := FakeRule("line", WithNeeds(NeedsLinePass))
	crossRule := FakeRule("cross", WithNeeds(NeedsCrossFile))

	if _, ok := ToV1(flatRule).(*V1FlatDispatch); !ok {
		t.Error("expected flat rule to produce V1FlatDispatch")
	}
	if _, ok := ToV1(lineRule).(*V1Line); !ok {
		t.Error("expected line rule to produce V1Line")
	}
	if _, ok := ToV1(crossRule).(*V1CrossFile); !ok {
		t.Error("expected cross-file rule to produce V1CrossFile")
	}
}
