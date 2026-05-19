package rules_test

// Selector coverage for nullability rules participating in the
// targeted-resolution pass under --depth=thorough. Each rule gets:
//   - unit tests that the selector returns the resolver-query positions
//     the rule's check() actually consumes,
//   - lexical-skip tests that prove the selector mirrors check()'s
//     short-circuit gates,
//   - an integration test that drives CollectExpressionPositions → fake
//     resolver → ApplyResolvedExpressions → dispatcher and asserts the
//     rule's verdict shifts once the seeded oracle fact arrives.

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// UnnecessaryNotNullCheck
// ---------------------------------------------------------------------------

// Property-declaration fixture: source-level resolver returns Unknown
// for `s` because `externalApi()` is undeclared in this translation
// unit, but the rule's flatReferenceHasSameFileTarget gate finds the
// `val s = ...` property and would consult the resolver. Seeding a
// non-null fact at the operand position lights up the rule's flag-and-fix
// path — the precision win that justifies adding the selector.
const unnecessaryNullCheckPropertyFixture = `
package test

class Probe {
    val s = externalApi()

    fun demo() {
        if (s != null) println(s)
    }
}
`

func TestUnnecessaryNotNullCheck_ExpressionPositions_ReturnsPropertyOperand(t *testing.T) {
	file := parseInline(t, unnecessaryNullCheckPropertyFixture)
	rule := findRuleByID(t, "UnnecessaryNotNullCheck")
	if rule.ExprPositions == nil {
		t.Fatal("UnnecessaryNotNullCheck should declare ExprPositions for thorough mode")
	}
	got := rule.ExprPositions(file)
	if len(got) != 1 {
		t.Fatalf("expected exactly one position for `s != null`; got %d: %v", len(got), got)
	}
	if text := file.FlatNodeText(got[0]); text != "s" {
		t.Errorf("selector returned non-`s` operand: %q", text)
	}
}

func TestUnnecessaryNotNullCheck_ExpressionPositions_SkipsLambdaParams(t *testing.T) {
	// Lambda parameters don't pass flatReferenceHasSameFileTarget — the
	// rule's check() exits before consulting the resolver, so seeding
	// KAA facts here would be wasted work. Selector must respect the
	// gate; the analogous UnnecessaryNotNullOperator selector uses
	// ResolveByNameFlat and does not have this gate.
	file := parseInline(t, `
package test

class Probe {
    fun demo() {
        listOf("a", "b").forEach { s ->
            if (s != null) println(s.length)
        }
    }
}
`)
	rule := findRuleByID(t, "UnnecessaryNotNullCheck")
	got := rule.ExprPositions(file)
	if len(got) != 0 {
		t.Fatalf("expected no positions for lambda-param operand (gate excludes them); got %d: %v",
			len(got), flatTextsForIndices(file, got))
	}
}

func TestUnnecessaryNotNullCheck_ExpressionPositions_SkipsNonNullComparisons(t *testing.T) {
	// `x == y` (no null literal) isn't a null check — selector must skip
	// or the targeted-resolution pre-pass will waste KAA work.
	file := parseInline(t, `
package test

class Probe {
    fun demo(a: Int, b: Int) {
        if (a == b) println("equal")
    }
}
`)
	rule := findRuleByID(t, "UnnecessaryNotNullCheck")
	got := rule.ExprPositions(file)
	if len(got) != 0 {
		t.Fatalf("expected no positions for non-null comparison; got %d: %v", len(got), got)
	}
}

func TestUnnecessaryNotNullCheck_ExpressionPositions_HandlesNilFile(t *testing.T) {
	rule := findRuleByID(t, "UnnecessaryNotNullCheck")
	if got := rule.ExprPositions(nil); got != nil {
		t.Errorf("expected nil for nil file; got %v", got)
	}
}

func TestUnnecessaryNotNullCheck_TargetedResolution_FiresOnPropertyFromUnknownInit(t *testing.T) {
	file := parseInline(t, unnecessaryNullCheckPropertyFixture)
	rule := findRuleByID(t, "UnnecessaryNotNullCheck")
	positions := api.CollectExpressionPositions([]*api.Rule{rule}, []*scanner.File{file})
	if len(positions[file.Path]) == 0 {
		t.Fatalf("expected a collected position for `s`; got %v", positions[file.Path])
	}
	fake := oracle.NewFakeOracle()
	seedExprFactsAtPositions(fake, file.Path, positions[file.Path],
		&typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass, Nullable: false})
	findings := runRuleOnFileWithFakeOracle(t, "UnnecessaryNotNullCheck", file, fake)
	if len(findings) != 1 {
		t.Fatalf("expected one finding once oracle fact is seeded; got %d: %v", len(findings), findings)
	}
	if findings[0].Rule != "UnnecessaryNotNullCheck" {
		t.Errorf("unexpected rule: %q", findings[0].Rule)
	}
}

// ---------------------------------------------------------------------------
// UnnecessarySafeCall
// ---------------------------------------------------------------------------

const unnecessarySafeCallLambdaParamFixture = `
package test

class Probe {
    fun demo() {
        listOf("a", "b").forEach { s ->
            println(s?.length)
        }
    }
}
`

func TestUnnecessarySafeCall_ExpressionPositions_ReturnsLambdaParamNav(t *testing.T) {
	file := parseInline(t, unnecessarySafeCallLambdaParamFixture)
	rule := findRuleByID(t, "UnnecessarySafeCall")
	if rule.ExprPositions == nil {
		t.Fatal("UnnecessarySafeCall should declare ExprPositions for thorough mode")
	}
	got := rule.ExprPositions(file)
	if len(got) != 1 {
		t.Fatalf("expected exactly one position for `s?.length`; got %d: %v", len(got), got)
	}
	if text := file.FlatNodeText(got[0]); text != "s?.length" {
		t.Errorf("selector returned unexpected node: %q", text)
	}
}

func TestUnnecessarySafeCall_ExpressionPositions_SkipsDottedReceivers(t *testing.T) {
	file := parseInline(t, `
package test

class Probe {
    fun demo(obj: Any?) {
        println(obj.foo?.length)
    }
}
`)
	rule := findRuleByID(t, "UnnecessarySafeCall")
	for _, idx := range rule.ExprPositions(file) {
		text := file.FlatNodeText(idx)
		// `obj.foo?.length` has a dotted receiver (`obj.foo`) — selector
		// must skip; only bare-name receivers are resolver-friendly.
		if text == "obj.foo?.length" {
			t.Errorf("selector should skip dotted receiver: %q", text)
		}
	}
}

func TestUnnecessarySafeCall_ExpressionPositions_HandlesNilFile(t *testing.T) {
	rule := findRuleByID(t, "UnnecessarySafeCall")
	if got := rule.ExprPositions(nil); got != nil {
		t.Errorf("expected nil for nil file; got %v", got)
	}
}

func TestUnnecessarySafeCall_TargetedResolution_FiresOnLambdaParam(t *testing.T) {
	file := parseInline(t, unnecessarySafeCallLambdaParamFixture)
	rule := findRuleByID(t, "UnnecessarySafeCall")
	positions := api.CollectExpressionPositions([]*api.Rule{rule}, []*scanner.File{file})
	if len(positions[file.Path]) == 0 {
		t.Fatalf("expected a collected position; got %v", positions[file.Path])
	}
	fake := oracle.NewFakeOracle()
	seedExprFactsAtPositions(fake, file.Path, positions[file.Path],
		&typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass, Nullable: false})
	findings := runRuleOnFileWithFakeOracle(t, "UnnecessarySafeCall", file, fake)
	if len(findings) != 1 {
		t.Fatalf("expected one finding once oracle fact is seeded; got %d: %v", len(findings), findings)
	}
	if findings[0].Rule != "UnnecessarySafeCall" {
		t.Errorf("unexpected rule: %q", findings[0].Rule)
	}
}

// NOTE: NullCheckOnMutableProperty deliberately does NOT declare
// ExprPositions. Its check() uses CompositeResolver.ResolveByNameFlat,
// which gives source the first word; the rule also only fires on
// same-owner `var` properties — shapes where source inference already
// has a precise nullability fact and KAA cannot improve the verdict.
// Seeding oracle facts for those positions would burn JVM work the
// rule can't read, so we leave the selector off and document why.

// ---------------------------------------------------------------------------
// NullableToStringCall
// ---------------------------------------------------------------------------

const nullableToStringLambdaParamFixture = `
package test

class Probe {
    fun demo(values: List<String?>) {
        values.forEach { v ->
            println(v.toString())
        }
    }
}
`

func TestNullableToStringCall_ExpressionPositions_ReturnsCallReceiver(t *testing.T) {
	file := parseInline(t, nullableToStringLambdaParamFixture)
	rule := findRuleByID(t, "NullableToStringCall")
	if rule.ExprPositions == nil {
		t.Fatal("NullableToStringCall should declare ExprPositions for thorough mode")
	}
	got := rule.ExprPositions(file)
	if len(got) == 0 {
		t.Fatal("expected at least one position for `v.toString()`")
	}
	sawV := false
	for _, idx := range got {
		if file.FlatNodeText(idx) == "v" {
			sawV = true
		}
	}
	if !sawV {
		t.Errorf("expected selector to include `v` (toString receiver); got %v",
			flatTextsForIndices(file, got))
	}
}

func TestNullableToStringCall_ExpressionPositions_SkipsSafeCallToString(t *testing.T) {
	// `v?.toString()` is already safe; the rule skips it, and so must the
	// selector — otherwise the targeted-resolution batch grows for no reason.
	file := parseInline(t, `
package test

class Probe {
    fun demo(v: String?) {
        println(v?.toString())
    }
}
`)
	rule := findRuleByID(t, "NullableToStringCall")
	for _, idx := range rule.ExprPositions(file) {
		if file.FlatNodeText(idx) == "v" {
			// `v` is a receiver of a safe-call toString — must NOT be selected.
			t.Errorf("selector should skip receivers of safe-call toString: %q", file.FlatNodeText(idx))
		}
	}
}

func TestNullableToStringCall_ExpressionPositions_ReturnsInterpolatedIdentifier(t *testing.T) {
	// `"$x"` triggers the string-template path; the selector must include
	// the interpolated identifier position so KAA can prove `x` is/isn't nullable.
	file := parseInline(t, `
package test

class Probe {
    fun demo(x: String?) {
        println("$x")
    }
}
`)
	rule := findRuleByID(t, "NullableToStringCall")
	got := rule.ExprPositions(file)
	sawX := false
	for _, idx := range got {
		if file.FlatType(idx) == "interpolated_identifier" {
			sawX = true
		}
	}
	if !sawX {
		t.Errorf("expected selector to include the `$x` interpolated_identifier; got %v",
			flatTextsForIndices(file, got))
	}
}

func TestNullableToStringCall_ExpressionPositions_HandlesNilFile(t *testing.T) {
	rule := findRuleByID(t, "NullableToStringCall")
	if got := rule.ExprPositions(nil); got != nil {
		t.Errorf("expected nil for nil file; got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// seedExprFactsAtPositions writes the same ResolvedType to every
// (line, col) position for a file in a FakeOracle, mirroring what
// ApplyResolvedExpressions does for a stub resolver that returned a
// single type for every requested position.
func seedExprFactsAtPositions(fake *oracle.FakeOracle, path string, positions []api.ExpressionPosition, t *typeinfer.ResolvedType) {
	if fake.Expressions[path] == nil {
		fake.Expressions[path] = map[string]*typeinfer.ResolvedType{}
	}
	for _, p := range positions {
		fake.Expressions[path][lineColKey(p.Line, p.Col)] = t
	}
}

// flatTextsForIndices is a debug helper for failure messages — turns a
// slice of flat indices into their source texts so the assertion message
// shows what the selector actually returned.
func flatTextsForIndices(file *scanner.File, idxs []uint32) []string {
	out := make([]string, 0, len(idxs))
	for _, idx := range idxs {
		out = append(out, file.FlatNodeText(idx))
	}
	return out
}
