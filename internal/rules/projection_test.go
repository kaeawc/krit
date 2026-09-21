package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestProjectDiagnostic_ProjectsMatchingOverlappingDiagnostic(t *testing.T) {
	file := parseInlineForInternalTest(t, "fun f() { val result = \"ready\" ?: \"fallback\" }\n")
	idx := firstFlatNodeOfType(t, file, "elvis_expression", `"ready" ?: "fallback"`)
	fake := oracle.NewFakeOracle()
	fake.Diagnostics[file.Path] = []oracle.Diagnostic{{
		FactoryName: "USELESS_ELVIS",
		Line:        7,
		Col:         11,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
	}}
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{
		File:      file,
		Idx:       idx,
		Resolver:  oracle.NewCompositeResolver(fake, typeinfer.NewResolver()),
		Collector: collector,
		Rule:      &api.Rule{ID: "Projected", Category: "test", Sev: api.SeverityWarning},
	}
	projected := projectDiagnostic(ctx, DiagnosticProjection{
		RuleID:       "Projected",
		FactoryNames: []string{"USELESS_ELVIS"},
		Message: func(*api.Context, oracle.Diagnostic) string {
			return "projected compiler verdict"
		},
		Fix: func(*api.Context, oracle.Diagnostic) *scanner.Fix {
			return &scanner.Fix{ByteMode: true, StartByte: 1, EndByte: 2, Replacement: "x"}
		},
		Confidence: api.ConfidenceVeryHigh,
	})
	if !projected {
		t.Fatal("expected matching diagnostic to project")
	}
	findings := collector.Columns().Findings()
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if got := findings[0]; got.Line != 7 || got.Col != 11 || got.Confidence != api.ConfidenceVeryHigh || got.Message != "projected compiler verdict" || got.Fix == nil {
		t.Fatalf("projected finding = %#v", got)
	}
}

func TestProjectDiagnostic_ReturnsFalseWithoutOracle(t *testing.T) {
	file := parseInlineForInternalTest(t, "fun f() { val result = \"ready\" ?: \"fallback\" }\n")
	idx := firstFlatNodeOfType(t, file, "elvis_expression", `"ready" ?: "fallback"`)
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{File: file, Idx: idx, Resolver: typeinfer.NewResolver(), Collector: collector}
	if projectDiagnostic(ctx, testDiagnosticProjection()) {
		t.Fatal("projection succeeded without a composite oracle")
	}
	if collector.Columns().Len() != 0 {
		t.Fatalf("findings = %d, want 0", collector.Columns().Len())
	}
}

func TestProjectDiagnostic_ReturnsFalseForNonOverlappingDiagnostic(t *testing.T) {
	file := parseInlineForInternalTest(t, "fun f() { val result = \"ready\" ?: \"fallback\" }\n")
	idx := firstFlatNodeOfType(t, file, "elvis_expression", `"ready" ?: "fallback"`)
	fake := oracle.NewFakeOracle()
	fake.Diagnostics[file.Path] = []oracle.Diagnostic{{
		FactoryName: "USELESS_ELVIS",
		StartByte:   0,
		EndByte:     1,
	}}
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{
		File:      file,
		Idx:       idx,
		Resolver:  oracle.NewCompositeResolver(fake, typeinfer.NewResolver()),
		Collector: collector,
	}
	if projectDiagnostic(ctx, testDiagnosticProjection()) {
		t.Fatal("projection succeeded for a non-overlapping diagnostic")
	}
	if collector.Columns().Len() != 0 {
		t.Fatalf("findings = %d, want 0", collector.Columns().Len())
	}
}

func testDiagnosticProjection() DiagnosticProjection {
	return DiagnosticProjection{
		FactoryNames: []string{"USELESS_ELVIS"},
		Message: func(*api.Context, oracle.Diagnostic) string {
			return "projected compiler verdict"
		},
		Confidence: api.ConfidenceVeryHigh,
	}
}
