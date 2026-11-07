package rules

import (
	"strings"
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

// TestProjectDiagnostic_NestedDiagnosticNotClaimedByEnclosingNode pins the fix
// for the mis-attribution bug: a USELESS_ELVIS emitted by the compiler on an
// inner elvis nested inside the right operand of an outer elvis must be claimed
// only by the inner node, never by the enclosing node whose byte range happens
// to contain it. A byte-range-overlap gate would (wrongly) let the outer node
// emit a false positive with a code-deleting fix.
func TestProjectDiagnostic_NestedDiagnosticNotClaimedByEnclosingNode(t *testing.T) {
	file := parseInlineForInternalTest(t, "fun f(x: String?) { val r = x ?: run { \"a\" ?: \"b\" } }\n")
	outer := firstFlatNodeOfType(t, file, "elvis_expression", "x ?: run { \"a\" ?: \"b\" }")
	inner := firstFlatNodeOfType(t, file, "elvis_expression", "\"a\" ?: \"b\"")

	fake := oracle.NewFakeOracle()
	// The compiler anchors USELESS_ELVIS on the inner elvis only.
	fake.Diagnostics[file.Path] = []oracle.Diagnostic{{
		FactoryName: "USELESS_ELVIS",
		StartByte:   int(file.FlatStartByte(inner)),
		EndByte:     int(file.FlatEndByte(inner)),
	}}
	resolver := oracle.NewCompositeResolver(fake, typeinfer.NewResolver())

	// Dispatched on the OUTER elvis: the nested diagnostic must not be claimed.
	outerCollector := scanner.NewFindingCollector(0)
	outerCtx := &api.Context{File: file, Idx: outer, Resolver: resolver, Collector: outerCollector}
	if projectDiagnostic(outerCtx, testDiagnosticProjection()) {
		t.Fatal("outer elvis wrongly claimed a diagnostic anchored on the inner elvis")
	}
	if outerCollector.Columns().Len() != 0 {
		t.Fatalf("outer findings = %d, want 0", outerCollector.Columns().Len())
	}

	// Dispatched on the INNER elvis: the diagnostic is claimed exactly once.
	innerCollector := scanner.NewFindingCollector(0)
	innerCtx := &api.Context{File: file, Idx: inner, Resolver: resolver, Collector: innerCollector}
	if !projectDiagnostic(innerCtx, testDiagnosticProjection()) {
		t.Fatal("inner elvis failed to claim its own diagnostic")
	}
	if innerCollector.Columns().Len() != 1 {
		t.Fatalf("inner findings = %d, want 1", innerCollector.Columns().Len())
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

func TestDiagnosticAnchorBytes_LineColFallbackCountsUTF16Units(t *testing.T) {
	// "é" is 2 bytes and 1 UTF-16 unit; "😀" is 4 bytes and 2 UTF-16 units.
	// Without byte offsets, the compiler's column must be converted, not
	// added to the line offset as bytes.
	src := "package p\nval s = \"é😀\"; val x = old()\n"
	file := parseInlineForInternalTest(t, src)
	idx := firstFlatNodeOfType(t, file, "call_expression", "old()")
	want := file.FlatStartByte(idx)
	// `val s = "é😀"; val x = ` is 23 UTF-16 units (31 bytes), so col 24.
	start, end, ok := diagnosticAnchorBytes(file, oracle.Diagnostic{Line: 2, Col: 24})
	if !ok || start != want || end != want {
		t.Fatalf("anchor = %d..%d ok=%v, want %d", start, end, ok, want)
	}
}

func TestUTF16ColumnToByteOffset(t *testing.T) {
	for _, tc := range []struct {
		line string
		col  int
		want int
	}{
		{"abc", 1, 0},
		{"abc", 3, 2},
		{"é_x", 2, 2},
		{"😀x", 3, 4},
		{"ab\ncd", 9, 2}, // past the end clamps to the newline
		{"ab", 9, 2},
	} {
		if got := utf16ColumnToByteOffset([]byte(tc.line), tc.col); got != tc.want {
			t.Errorf("utf16ColumnToByteOffset(%q, %d) = %d, want %d", tc.line, tc.col, got, tc.want)
		}
	}
}

func TestCompilerAnalyzedFile_SkipsHeuristicForUnnecessarySafeCall(t *testing.T) {
	src := "fun check() {\n    val name: String = \"hello\"\n    val len = name?.length\n}\n"
	run := func(analyzed bool) int {
		file := parseInlineForInternalTest(t, src)
		idx := firstFlatNodeOfType(t, file, "navigation_expression", "name?.length")
		fake := oracle.NewFakeOracle()
		fake.AnalyzedFiles[file.Path] = analyzed
		collector := scanner.NewFindingCollector(0)
		ctx := &api.Context{
			File:      file,
			Idx:       idx,
			Resolver:  oracle.NewCompositeResolver(fake, typeinfer.NewResolver()),
			Collector: collector,
			Rule:      &api.Rule{ID: "UnnecessarySafeCall", Category: "potential-bugs", Sev: api.SeverityWarning},
		}
		(&UnnecessarySafeCallRule{BaseRule: BaseRule{RuleName: "UnnecessarySafeCall"}}).check(ctx)
		return len(collector.Columns().Findings())
	}
	if got := run(true); got != 0 {
		t.Fatalf("compiler-analyzed file with no diagnostic produced %d findings, want 0", got)
	}
}

func TestProjectDiagnostic_DropsSmartCastVerdictOnAnotherModulesProperty(t *testing.T) {
	src := "fun f(account: Account): String? {\n    if (account.profile != null) return account.profile?.name\n    return null\n}\n"
	const decl = "/repo/lib/src/main/kotlin/Account.kt"
	run := func(moduleOf func(string) string) int {
		file := parseInlineForInternalTest(t, src)
		idx := firstFlatNodeOfType(t, file, "navigation_expression", "account.profile?.name")
		safe := strings.Index(string(file.Content), "?.")
		fake := oracle.NewFakeOracle()
		fake.Diagnostics[file.Path] = []oracle.Diagnostic{{
			FactoryName:    "UNNECESSARY_SAFE_CALL",
			Line:           2,
			Col:            48,
			StartByte:      safe,
			EndByte:        safe + 2,
			SmartCastDecls: []string{decl},
		}}
		resolver := oracle.NewCompositeResolver(fake, typeinfer.NewResolver())
		if moduleOf != nil {
			resolver.SetModuleOf(func(path string) string {
				if path == file.Path {
					return ":app"
				}
				return moduleOf(path)
			})
		}
		collector := scanner.NewFindingCollector(0)
		ctx := &api.Context{
			File:      file,
			Idx:       idx,
			Resolver:  resolver,
			Collector: collector,
			Rule:      &api.Rule{ID: "UnnecessarySafeCall", Category: "potential-bugs", Sev: api.SeverityWarning},
		}
		(&UnnecessarySafeCallRule{BaseRule: BaseRule{RuleName: "UnnecessarySafeCall"}}).check(ctx)
		return len(collector.Columns().Findings())
	}
	// Another module: a per-module build rejects the smart cast, and the heuristic stays silent on an analyzed file.
	if got := run(func(string) string { return ":lib" }); got != 0 {
		t.Fatalf("smart cast across modules produced %d findings, want 0", got)
	}
	// The same module, or a friend source set sharing its key, keeps the compiler's verdict.
	if got := run(func(string) string { return ":app" }); got != 1 {
		t.Fatalf("smart cast within the module produced %d findings, want 1", got)
	}
	// An unknown declaring module is unproven, so the smart-cast verdict is dropped.
	if got := run(func(string) string { return "" }); got != 0 {
		t.Fatalf("unknown declaring module produced %d findings, want 0", got)
	}
	// No module map behaves as a whole-repo compile.
	if got := run(nil); got != 1 {
		t.Fatalf("no module map produced %d findings, want 1", got)
	}
}
