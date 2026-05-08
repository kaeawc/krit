package rules_test

// Evidence tests for the "depth=thorough" investigation. These tests
// pin down which precision wins are realistically reachable by
// extending the oracle and which are blocked by Go-side resolver
// wiring rather than by KAA fact coverage.
//
// Each test uses the FakeOracle to seed exactly the new fact being
// proposed (e.g. "lambda parameter type at byte range X"), runs the
// real rule through the dispatcher, and asserts what the rule does.
// A failing test here means "even with the proposed new fact, the
// rule's current code path can't see it" — useful negative evidence
// when ranking implementation work.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runRuleWithFakeOracle executes ruleName against code using a
// CompositeResolver that wraps the real source-level resolver with the
// supplied FakeOracle. Returns the dispatcher's emitted findings so
// tests can assert FN/FP shifts caused by oracle facts.
func runRuleWithFakeOracle(t *testing.T, ruleName, code string, fake *oracle.FakeOracle) (*scanner.File, []scanner.Finding) {
	t.Helper()
	file := parseInline(t, code)
	return file, runRuleOnFileWithFakeOracle(t, ruleName, file, fake)
}

// runRuleOnFileWithFakeOracle runs the rule against an already-parsed file
// so callers can seed the FakeOracle keyed on the same file path before
// dispatching. Splitting parsing from dispatch is required because the
// FakeOracle keys expression facts by file.Path.
func runRuleOnFileWithFakeOracle(t *testing.T, ruleName string, file *scanner.File, fake *oracle.FakeOracle) []scanner.Finding {
	t.Helper()
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

// findFlatNodeOf returns the FlatNode index for the first node whose
// kind matches and whose text equals want. Used to position oracle
// facts at the exact byte range the rule's resolver call will hit.
func findFlatNodeOf(t *testing.T, file *scanner.File, kind, want string) (uint32, bool) {
	t.Helper()
	var found uint32
	ok := false
	file.FlatWalkNodes(0, kind, func(idx uint32) {
		if ok {
			return
		}
		if strings.TrimSpace(file.FlatNodeText(idx)) == want {
			found = idx
			ok = true
		}
	})
	return found, ok
}

// Hypothesis 1: UnnecessaryNotNullOperator misses lambda-param `!!`.
//
// The rule resolves the operand via ctx.Resolver.ResolveByNameFlat(name).
// The source-level resolver cannot trace `s` to forEach<String>'s
// parameter type, so it returns nil/Unknown and the rule bails out.
//
// This first test pins the current behavior — no finding — without
// any oracle facts. It documents the FN.
func TestEvidence_UnnecessaryNotNullOperator_LambdaParam_NoFindingWithoutOracle(t *testing.T) {
	fake := oracle.NewFakeOracle()
	_, findings := runRuleWithFakeOracle(t, "UnnecessaryNotNullOperator", `
package test

class Probe {
    fun demo() {
        listOf("a", "b").forEach { s ->
            println(s!!.length)
        }
    }
}
`, fake)
	if len(findings) != 0 {
		t.Fatalf("baseline FN test: expected no finding (source resolver can't type lambda param 's'); got %d: %v",
			len(findings), findings)
	}
}

// Hypothesis 1b: with CompositeResolver.ResolveByNameFlat now consulting
// LookupExpressionFlat after the source fallback returns nil, seeding the
// FakeOracle with the precise non-nullable type for the lambda param `s`
// makes the rule fire. This pins the precision win unlocked by the
// resolver-wiring change.
func TestEvidence_UnnecessaryNotNullOperator_LambdaParam_FiresWithOracleExprFact(t *testing.T) {
	code := `
package test

class Probe {
    fun demo() {
        listOf("a", "b").forEach { s ->
            println(s!!.length)
        }
    }
}
`
	file := parseInline(t, code)
	// Seed at the postfix_expression position — that's the idx the rule
	// passes to ResolveByNameFlat, and FakeOracle.LookupExpressionFlat
	// keys on (row+1, col+1) of that idx.
	idx, ok := findFlatNodeOf(t, file, "postfix_expression", "s!!")
	if !ok {
		t.Fatal("could not locate the `s!!` postfix expression in the parsed fixture")
	}
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{
		positionKey(file, idx): {Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass, Nullable: false},
	}
	findings := runRuleOnFileWithFakeOracle(t, "UnnecessaryNotNullOperator", file, fake)
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding once oracle expression fact is consulted; got %d: %v",
			len(findings), findings)
	}
	if findings[0].Rule != "UnnecessaryNotNullOperator" {
		t.Fatalf("expected Rule=UnnecessaryNotNullOperator; got %q", findings[0].Rule)
	}
}

// Hypothesis 2 (FALSIFIED): CharArrayToStringCall was suspected to
// under-emit on parameter-typed CharArray receivers. In fact the rule
// fires today at confidence 0.9 on parameter receivers — the source
// resolver already types parameter-declared CharArrays correctly via
// `chars: CharArray` syntax inspection. This is a documented null
// result; ranking dropped CharArrayToStringCall as a thorough-mode
// candidate.
func TestEvidence_CharArrayToStringCall_ParamReceiver_AlreadyHandled(t *testing.T) {
	fake := oracle.NewFakeOracle()
	_, findings := runRuleWithFakeOracle(t, "CharArrayToStringCall", `
package test

class Probe {
    fun render(chars: CharArray): String {
        return chars.toString()
    }
}
`, fake)
	if len(findings) == 0 {
		t.Fatalf("regression check: rule should fire at confidence 0.9 on parameter receiver; got 0 findings")
	}
	if findings[0].Confidence < 0.9 {
		t.Fatalf("regression check: confidence dropped from 0.9 to %v; rule's resolver path may be broken",
			findings[0].Confidence)
	}
}

// Hypothesis 3 (FALSIFIED in current shape): UnsafeCast on an external
// type was suspected to depend on richer class-hierarchy facts.
// In practice the rule does not flag `obj as Fragment` regardless of
// whether the Fragment class fact is registered in the oracle —
// confirmed by both runs returning 0 findings. The decision tree is
// gated on different signals (e.g. specific framework patterns,
// nullable casts), not on raw "is the target class final."
//
// Single combined test instead of paired baseline/with-fact —
// asserting the symmetric null result is enough evidence to drop
// UnsafeCast as a thorough-mode candidate without further investigation.
func TestEvidence_UnsafeCast_ExternalTargetType_ClassFactDoesNotChangeVerdict(t *testing.T) {
	fixture := `
package test

import androidx.fragment.app.Fragment

class Probe {
    fun toFrag(obj: Any): Fragment {
        return obj as Fragment
    }
}
`
	_, baseline := runRuleWithFakeOracle(t, "UnsafeCast", fixture, oracle.NewFakeOracle())

	withFact := oracle.NewFakeOracle()
	withFact.Classes["Fragment"] = &typeinfer.ClassInfo{
		Name: "Fragment", FQN: "androidx.fragment.app.Fragment",
		Kind: "class", IsAbstract: false, IsOpen: true,
	}
	withFact.Classes["androidx.fragment.app.Fragment"] = withFact.Classes["Fragment"]
	_, withClass := runRuleWithFakeOracle(t, "UnsafeCast", fixture, withFact)

	if len(baseline) != len(withClass) {
		t.Fatalf("hypothesis revisit: counts diverged (baseline=%d, withClass=%d) — "+
			"investigate whether richer class facts could now improve UnsafeCast",
			len(baseline), len(withClass))
	}
	if len(baseline) != 0 {
		t.Logf("note: rule emits %d finding(s) on this fixture even without oracle facts; "+
			"adjust fixture if you want to test the no-emit branch", len(baseline))
	}
}

// positionKey is the "line:col" string the FakeOracle uses to store
// expression facts. Mirrors LookupExpression's call signature.
func positionKey(file *scanner.File, idx uint32) string {
	return fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
}
