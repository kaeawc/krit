package rules_test

// Regression tests for false positives the FIR/KAA oracle introduced into the
// redundant-null-safety rules by reporting structurally-nullable expressions
// (safe casts `x as? T`, Map indexed access `m[k]`) as non-null. Each test
// seeds the FakeOracle with the bad non-null fact the real backend produced and
// asserts the rule no longer flags. Without the structural guards these would
// fire — the source-only fixtures cannot reproduce them because the FP only
// appears once an oracle fact is consulted.

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// TestRegression_CastNullable_ByteRangeFactBeatsColumnDrift locks the
// CastNullableToNonNullableType byte-range fix: the rule must consult the
// oracle's smart-cast nullability by BYTE RANGE, not line/col. FIR emits
// columns that don't line up with tree-sitter, so a line/col probe misses the
// fact and falls back to source inference (which sees `o: Any?` as nullable and
// wrongly flags the guarded cast). This seeds a real oracle with a non-null
// fact at the operand's byte range but a DRIFTED line:col key — only the
// byte-range path finds it, so the cast must not be flagged.
func TestRegression_CastNullable_ByteRangeFactBeatsColumnDrift(t *testing.T) {
	file := parseInline(t, "fun f(o: Any?): String { return o as String }\n")
	operand := castOperandNode(t, file)
	o, err := oracle.LoadFromData(&oracle.Data{
		Version:       1,
		KotlinVersion: "2.1.0",
		Files: map[string]*oracle.File{
			file.Path: {Expressions: map[string]*oracle.ExpressionType{
				// Deliberately-wrong line:col; correct byte range.
				"999:999": {Type: "kotlin.Any", Nullable: false, StartByte: int(file.FlatStartByte(operand)), EndByte: int(file.FlatEndByte(operand))},
			}},
		},
		Dependencies: map[string]*oracle.Class{},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	composite := oracle.NewCompositeResolver(o, resolver)
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "CastNullableToNonNullableType" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("CastNullableToNonNullableType not registered")
	}
	cols := rules.NewDispatcher([]*api.Rule{rule}, composite).Run(file)
	findings := cols.Findings()
	for _, f := range findings {
		if f.Rule == "CastNullableToNonNullableType" {
			t.Fatalf("byte-range non-null (smart-cast) fact should suppress the cast; got FP: %v", f)
		}
	}
}

func nonNullFact(name, fqn string) *typeinfer.ResolvedType {
	return &typeinfer.ResolvedType{Name: name, FQN: fqn, Kind: typeinfer.TypeClass, Nullable: false}
}

// castOperandNode returns the operand node of the first `as_expression`
// (the source expression being cast), i.e. the `x` in `x as String` — not
// the same-named parameter declaration that a text search would hit first.
func castOperandNode(t *testing.T, file *scanner.File) uint32 {
	t.Helper()
	var operand uint32
	file.FlatWalkNodes(0, "as_expression", func(i uint32) {
		if operand == 0 {
			operand = file.FlatChild(i, 0)
		}
	})
	if operand == 0 {
		t.Fatal("no as_expression operand found")
	}
	return operand
}

// TestRegression_UselessElvis_SafeCastNotFlagged: `(x as? Int) ?: 0` — the safe
// cast result is always nullable, so the elvis fallback is live, not dead.
func TestRegression_UselessElvis_SafeCastNotFlagged(t *testing.T) {
	code := "fun f(x: Any): Int { return (x as? Int) ?: 0 }\n"
	file := parseInline(t, code)
	idx, ok := findFlatNodeOf(t, file, "as_expression", "x as? Int")
	if !ok {
		t.Fatal("could not locate the `x as? Int` as_expression")
	}
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{
		positionKey(file, idx): nonNullFact("Int", "kotlin.Int"),
	}
	findings := runRuleOnFileWithFakeOracle(t, "UselessElvisOnNonNull", file, fake)
	if len(findings) != 0 {
		t.Fatalf("safe-cast elvis must not be flagged even when the oracle reports the cast non-null; got %d: %v", len(findings), findings)
	}
}

// TestRegression_UnnecessarySafeCall_SafeCastReceiverNotFlagged:
// `(x as? Foo)?.bar()` — the safe-cast receiver is nullable, so `?.` is needed.
func TestRegression_UnnecessarySafeCall_SafeCastReceiverNotFlagged(t *testing.T) {
	code := "fun f(x: Any) { (x as? String)?.length }\n"
	file := parseInline(t, code)
	idx, ok := findFlatNodeOf(t, file, "navigation_expression", "(x as? String)?.length")
	if !ok {
		t.Fatal("could not locate the navigation_expression")
	}
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{
		positionKey(file, idx): nonNullFact("String", "kotlin.String"),
	}
	findings := runRuleOnFileWithFakeOracle(t, "UnnecessarySafeCall", file, fake)
	if len(findings) != 0 {
		t.Fatalf("safe-cast receiver safe call must not be flagged; got %d: %v", len(findings), findings)
	}
}

// TestRegression_UnnecessaryNotNullOperator_MapIndexNotFlagged:
// `m[k]!!` — Map.get returns a nullable value, so the `!!` is required.
func TestRegression_UnnecessaryNotNullOperator_MapIndexNotFlagged(t *testing.T) {
	code := "fun f(m: Map<String, Int>) { val v = m[\"k\"]!! }\n"
	file := parseInline(t, code)
	idx, ok := findFlatNodeOf(t, file, "postfix_expression", "m[\"k\"]!!")
	if !ok {
		t.Fatal("could not locate the `m[\"k\"]!!` postfix expression")
	}
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{
		positionKey(file, idx): nonNullFact("Int", "kotlin.Int"),
	}
	findings := runRuleOnFileWithFakeOracle(t, "UnnecessaryNotNullOperator", file, fake)
	if len(findings) != 0 {
		t.Fatalf("`!!` on a Map indexed access must not be flagged; got %d: %v", len(findings), findings)
	}
}

// TestRegression_CastNullableToNonNullable_SmartCastOperandNotFlagged:
// `x as T` where the oracle (via krit-fir's smart-cast fact) reports the
// operand `x` as NON-null at the cast site must not be flagged — the cast is
// not a nullable→non-null cast. This is the equals()/guarded-cast idiom
// (`if (x == null) return; x as T`). The rule must read the oracle's
// (byte-range) nullability for the operand, not source inference's declared
// type.
func TestRegression_CastNullableToNonNullable_SmartCastOperandNotFlagged(t *testing.T) {
	code := "fun f(x: Any?) { val y = x as String }\n"
	file := parseInline(t, code)
	idx := castOperandNode(t, file)
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{
		positionKey(file, idx): nonNullFact("String", "kotlin.String"),
	}
	findings := runRuleOnFileWithFakeOracle(t, "CastNullableToNonNullableType", file, fake)
	if len(findings) != 0 {
		t.Fatalf("cast of a smart-cast non-null operand must not be flagged; got %d: %v", len(findings), findings)
	}
}

// TestRegression_CastNullableToNonNullable_NullableOperandStillFlagged is the
// positive control: when the oracle reports the operand as nullable, the cast
// IS a nullable→non-null cast and must still be flagged.
func TestRegression_CastNullableToNonNullable_NullableOperandStillFlagged(t *testing.T) {
	code := "fun f(x: Any?) { val y = x as String }\n"
	file := parseInline(t, code)
	idx := castOperandNode(t, file)
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{
		positionKey(file, idx): {Name: "Any", FQN: "kotlin.Any", Kind: typeinfer.TypeNullable, Nullable: true},
	}
	findings := runRuleOnFileWithFakeOracle(t, "CastNullableToNonNullableType", file, fake)
	if len(findings) != 1 {
		t.Fatalf("cast of a nullable operand should still be flagged; got %d: %v", len(findings), findings)
	}
}

// TestRegression_UnnecessarySafeCall_MapIndexReceiverNotFlagged:
// `m[k]?.foo` — indexed Map receiver is nullable, so `?.` is needed.
func TestRegression_UnnecessarySafeCall_MapIndexReceiverNotFlagged(t *testing.T) {
	code := "fun f(m: Map<String, String>) { m[\"k\"]?.length }\n"
	file := parseInline(t, code)
	idx, ok := findFlatNodeOf(t, file, "navigation_expression", "m[\"k\"]?.length")
	if !ok {
		t.Fatal("could not locate the navigation_expression")
	}
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{
		positionKey(file, idx): nonNullFact("String", "kotlin.String"),
	}
	findings := runRuleOnFileWithFakeOracle(t, "UnnecessarySafeCall", file, fake)
	if len(findings) != 0 {
		t.Fatalf("safe call on a Map indexed receiver must not be flagged; got %d: %v", len(findings), findings)
	}
}
