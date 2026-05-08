package rules_test

// Tests for the UnnecessaryNotNullOperator ExpressionPositions selector
// added in PR E (lights up the targeted-resolution path under
// --depth=thorough). Two layers of coverage:
//
//   1. Unit test of the selector — proves it returns the postfix_expression
//      indices for `!!` patterns the rule would query.
//
//   2. Integration test mirroring the production pipeline path —
//      CollectExpressionPositions → fake resolver → ApplyResolvedExpressions
//      → run rule via dispatcher. Asserts the rule fires on lambda-param
//      `s!!` once the oracle has the type fact, proving the full plumbing
//      from selector to finding works end to end.

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const lambdaParamBangBangFixture = `
package test

class Probe {
    fun demo() {
        listOf("a", "b").forEach { s ->
            println(s!!.length)
        }
    }
}
`

func TestUnnecessaryNotNullOperator_ExpressionPositions_ReturnsLambdaParamPostfix(t *testing.T) {
	file := parseInline(t, lambdaParamBangBangFixture)
	rule := findUnnecessaryNotNullOperatorRule(t)
	if rule.ExprPositions == nil {
		t.Fatal("UnnecessaryNotNullOperator should declare ExprPositions for thorough mode")
	}

	got := rule.ExprPositions(file)
	if len(got) != 1 {
		t.Fatalf("expected exactly one position for the lambda-param `s!!`; got %d: %v", len(got), got)
	}
	if text := file.FlatNodeText(got[0]); text != "s!!" {
		t.Errorf("selector returned non-`s!!` postfix: %q", text)
	}
}

func TestUnnecessaryNotNullOperator_ExpressionPositions_SkipsDottedReceivers(t *testing.T) {
	// Dotted receivers (`obj.field!!`) can't be resolved by bare-name lookup,
	// so the rule's check method skips them and the selector should too —
	// otherwise we'd waste KAA resolution on positions the rule won't read.
	file := parseInline(t, `
package test

class Probe {
    fun demo(obj: Any?) {
        println(obj!!.toString())
        println((obj as String).length)
    }
}
`)
	rule := findUnnecessaryNotNullOperatorRule(t)
	got := rule.ExprPositions(file)
	for _, idx := range got {
		text := file.FlatNodeText(idx)
		// `obj!!` is bare so it should be selected; `obj.toString()` and
		// `(obj as String).length` aren't postfix-`!!` so shouldn't appear.
		if text != "obj!!" {
			t.Errorf("unexpected position selected: %q", text)
		}
	}
}

func TestUnnecessaryNotNullOperator_ExpressionPositions_HandlesNilFile(t *testing.T) {
	rule := findUnnecessaryNotNullOperatorRule(t)
	if got := rule.ExprPositions(nil); got != nil {
		t.Errorf("expected nil for nil file; got %v", got)
	}
}

// Integration: drive CollectExpressionPositions → fake resolver →
// ApplyResolvedExpressions → dispatcher, mirroring what
// pipeline.RunTargetedResolutionPass does in production. Proves the
// selector + v2 helpers + dispatcher all chain together to surface
// the precision win on lambda-param `!!`.
func TestUnnecessaryNotNullOperator_TargetedResolution_FiresOnLambdaParam(t *testing.T) {
	file := parseInline(t, lambdaParamBangBangFixture)
	rule := findUnnecessaryNotNullOperatorRule(t)

	// Step 1: collect positions just like the production pre-pass would.
	positions := api.CollectExpressionPositions([]*api.Rule{rule}, []*scanner.File{file})
	if len(positions[file.Path]) != 1 {
		t.Fatalf("expected exactly one collected position; got %v", positions[file.Path])
	}
	pos := positions[file.Path][0]

	// Step 2: simulate the daemon resolving that position to a non-null String.
	resolver := stubExprResolver{result: map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType{
		file.Path: {pos: {Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass, Nullable: false}},
	}}
	results, err := resolver.Resolve(positions)
	if err != nil {
		t.Fatalf("stub resolver error: %v", err)
	}

	// Step 3: apply to a FakeOracle that the dispatcher will see via
	// CompositeResolver.ResolveByNameFlat.
	fake := oracle.NewFakeOracle()
	api.ApplyResolvedExpressions(fakeOracleSink{fake}, results)

	// Step 4: run the rule and assert the finding fires.
	findings := runRuleOnFileWithFakeOracle(t, "UnnecessaryNotNullOperator", file, fake)
	if len(findings) != 1 {
		t.Fatalf("expected one finding once the targeted-resolution chain populates the oracle; got %d: %v",
			len(findings), findings)
	}
	if findings[0].Rule != "UnnecessaryNotNullOperator" {
		t.Errorf("unexpected rule: %q", findings[0].Rule)
	}
}

// stubExprResolver is a minimal api.ExpressionTypeResolver for tests —
// returns whatever was canned. Mirrors the fake in
// internal/pipeline/exprresolve_test.go but local here to avoid
// cross-package _test imports.
type stubExprResolver struct {
	result map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType
	err    error
}

func (r stubExprResolver) Resolve(_ map[string][]api.ExpressionPosition) (map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType, error) {
	return r.result, r.err
}

// fakeOracleSink adapts FakeOracle to api.ExpressionFactSink by writing
// resolved facts into the FakeOracle.Expressions map at the same
// "line:col" key shape LookupExpression reads.
type fakeOracleSink struct{ fake *oracle.FakeOracle }

func (s fakeOracleSink) SetExpressionFact(filePath string, line, col int, t *typeinfer.ResolvedType) {
	if s.fake.Expressions[filePath] == nil {
		s.fake.Expressions[filePath] = map[string]*typeinfer.ResolvedType{}
	}
	s.fake.Expressions[filePath][lineColKey(line, col)] = t
}

func lineColKey(line, col int) string {
	return itoaQuick(line) + ":" + itoaQuick(col)
}

func itoaQuick(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func findUnnecessaryNotNullOperatorRule(t *testing.T) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == "UnnecessaryNotNullOperator" {
			return r
		}
	}
	t.Fatal("UnnecessaryNotNullOperator not in api.Registry")
	return nil
}
