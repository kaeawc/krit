package rules

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// deprecationAnchor places a fake DEPRECATION diagnostic on the nth (0-based)
// node of kind whose text is text — the identifier token K2 anchors on.
type deprecationAnchor struct {
	kind, text string
	nth        int
	message    string
}

func nthFlatNodeWithText(t *testing.T, file *scanner.File, kind, text string, nth int) uint32 {
	t.Helper()
	var hits []uint32
	file.FlatWalkNodes(0, kind, func(idx uint32) {
		if strings.TrimSpace(file.FlatNodeText(idx)) == text {
			hits = append(hits, idx)
		}
	})
	if nth >= len(hits) {
		t.Fatalf("want %s %q #%d, found %d", kind, text, nth, len(hits))
	}
	return hits[nth]
}

// runDeprecation runs the registered Deprecation rule over code. With anchors
// it resolves through a composite oracle carrying those DEPRECATION
// diagnostics; with nil it runs source-only, exercising the fallback paths.
func runDeprecation(t *testing.T, code string, anchors []deprecationAnchor) []scanner.Finding {
	t.Helper()
	file := parseInlineForInternalTest(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	var r typeinfer.TypeResolver = resolver
	if anchors != nil {
		fake := oracle.NewFakeOracle()
		for _, a := range anchors {
			idx := nthFlatNodeWithText(t, file, a.kind, a.text, a.nth)
			msg := a.message
			if msg == "" {
				msg = "'" + a.text + "' is deprecated."
			}
			fake.Diagnostics[file.Path] = append(fake.Diagnostics[file.Path], oracle.Diagnostic{
				FactoryName: "DEPRECATION",
				Severity:    "WARNING",
				Message:     msg,
				Line:        file.FlatRow(idx) + 1,
				Col:         file.FlatCol(idx) + 1,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
			})
		}
		r = oracle.NewCompositeResolver(fake, resolver)
	}
	for _, rule := range api.Registry {
		if rule.ID == "Deprecation" {
			cols := NewDispatcher([]*api.Rule{rule}, r).Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("Deprecation rule not registered")
	return nil
}

func TestDeprecationProjection_ClaimsEachDiagnosticOnceAcrossNestedReferences(t *testing.T) {
	// `a.dep.m()` is a call_expression wrapping a navigation_expression, and the
	// inner `a.dep` navigation is dispatched too. Both cover the span of the
	// diagnostic on `dep`; only the node whose own name token it is may claim it.
	code := "package p\nfun use(a: A) {\n    a.dep.m()\n}\n"
	findings := runDeprecation(t, code, []deprecationAnchor{
		{kind: "simple_identifier", text: "dep"},
		{kind: "simple_identifier", text: "m"},
	})
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2 (one per diagnostic): %#v", len(findings), findings)
	}
	cols := map[int]bool{}
	for _, f := range findings {
		if f.Confidence != api.ConfidenceVeryHigh {
			t.Errorf("confidence = %v, want projected %v", f.Confidence, api.ConfidenceVeryHigh)
		}
		cols[f.Col] = true
	}
	// `    a.dep.m()` — dep starts at col 7, m at col 11.
	if !cols[7] || !cols[11] {
		t.Fatalf("finding cols = %v, want {7, 11} (dep and m)", cols)
	}
}

func TestDeprecationProjection_TypeReference(t *testing.T) {
	findings := runDeprecation(t, "package p\nfun f(x: OldType) {}\n", []deprecationAnchor{
		{kind: "type_identifier", text: "OldType"},
	})
	if len(findings) != 1 || findings[0].Confidence != api.ConfidenceVeryHigh {
		t.Fatalf("findings = %#v, want one projected type-reference finding", findings)
	}
}

func TestDeprecationProjection_OnlyTheDiagnosedOverloadCall(t *testing.T) {
	// The deprecated overload is declared in another file, so only the
	// compiler knows which call resolves to it.
	findings := runDeprecation(t, "package p\nfun use() {\n    over(1)\n    over(\"x\")\n}\n", []deprecationAnchor{
		{kind: "simple_identifier", text: "over", nth: 1},
	})
	if len(findings) != 1 || findings[0].Line != 4 {
		t.Fatalf("findings = %#v, want exactly the line-4 over(\"x\") call", findings)
	}
}

func TestDeprecationProjection_MessageCarriesCompilerDetail(t *testing.T) {
	findings := runDeprecation(t, "package p\nfun use() { oldFn() }\n", []deprecationAnchor{
		{kind: "simple_identifier", text: "oldFn", message: "'fun oldFn(): Unit' is deprecated. use newFn."},
	})
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want 1", findings)
	}
	if got, want := findings[0].Message, "'oldFn' is deprecated: use newFn"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestDeprecation_SourceFallbackUnchangedWithoutOracle(t *testing.T) {
	code := `package p
@Deprecated("old", ReplaceWith("newFn()"))
fun oldFn() {}
fun newFn() {}
fun use() { oldFn() }
`
	findings := runDeprecation(t, code, nil)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want 1 from the same-file path", findings)
	}
	f := findings[0]
	if f.Confidence == api.ConfidenceVeryHigh {
		t.Errorf("source fallback must not claim compiler-verdict confidence")
	}
	if !strings.HasPrefix(f.Message, "'oldFn' is deprecated") {
		t.Errorf("message = %q", f.Message)
	}
	if f.Fix == nil || f.Fix.Replacement != "newFn()" {
		t.Errorf("fix = %#v, want the ReplaceWith expression", f.Fix)
	}
}

func TestCompilerDeprecationDetail(t *testing.T) {
	cases := map[string]string{
		"'fun oldFn(): Unit' is deprecated. use newFn.":      "use newFn",
		"'var year: Int' is deprecated. Deprecated in Java.": "Deprecated in Java",
		"'class Old : Any' is deprecated.":                   "",
		"Identity equality for arguments is deprecated.":     "",
	}
	for in, want := range cases {
		if got := compilerDeprecationDetail(in); got != want {
			t.Errorf("compilerDeprecationDetail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeprecationProjection_KeepsSameFileFixForTheFlaggedDeclaration(t *testing.T) {
	code := `package p
@Deprecated("use newFn", ReplaceWith("newFn()"))
fun oldFn() {}
fun newFn() {}
fun use() { oldFn() }
`
	findings := runDeprecation(t, code, []deprecationAnchor{
		{kind: "simple_identifier", text: "oldFn", nth: 1, message: "'fun oldFn(): Unit' is deprecated. use newFn."},
	})
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want 1", findings)
	}
	f := findings[0]
	if f.Confidence != api.ConfidenceVeryHigh || f.Fix == nil || f.Fix.Replacement != "newFn()" {
		t.Fatalf("finding = %#v, want projected with the declaration's ReplaceWith fix", f)
	}
}

func TestDeprecationProjection_SameNamedOtherDeclarationGetsNoBorrowedFix(t *testing.T) {
	// A.run is deprecated in this file, but the compiler flagged b.run(),
	// which resolves to a different run() (another class, or a library) with
	// its own deprecation. The same-file entry must lend neither its message
	// nor its ReplaceWith.
	code := `package p
class A {
    @Deprecated("local reason", ReplaceWith("newLocal()"))
    fun run() {}
}
fun use(b: B) { b.run() }
`
	findings := runDeprecation(t, code, []deprecationAnchor{
		{kind: "simple_identifier", text: "run", nth: 1, message: "'fun run(): Unit' is deprecated. library reason."},
	})
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want 1", findings)
	}
	f := findings[0]
	if f.Message != "'run' is deprecated: library reason" {
		t.Fatalf("message = %q, want the compiler's reason", f.Message)
	}
	if f.Fix != nil {
		t.Fatalf("fix = %#v; a different declaration's ReplaceWith must not be applied", f.Fix)
	}
}
