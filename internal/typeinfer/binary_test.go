package typeinfer

import (
	"testing"
)

func TestBinaryExpr_IntAddition(t *testing.T) {
	src := `val result = 1 + 2`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "additive_expression")
	if idx == 0 {
		t.Fatal("expected to find additive_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "Int" {
		t.Errorf("expected Int, got %v", got)
	}
}

func TestBinaryExpr_StringConcatenation(t *testing.T) {
	src := `val result = "a" + "b"`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "additive_expression")
	if idx == 0 {
		t.Fatal("expected to find additive_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "String" {
		t.Errorf("expected String, got %v", got)
	}
}

func TestBinaryExpr_DoubleMultiplication(t *testing.T) {
	src := `val result = 1.5 * 2.0`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "multiplicative_expression")
	if idx == 0 {
		t.Fatal("expected to find multiplicative_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "Double" {
		t.Errorf("expected Double, got %v", got)
	}
}

func TestBinaryExpr_Comparison(t *testing.T) {
	src := `val result = 1 > 0`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "comparison_expression")
	if idx == 0 {
		t.Fatal("expected to find comparison_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "Boolean" {
		t.Errorf("expected Boolean, got %v", got)
	}
}

func TestBinaryExpr_EqualityNull(t *testing.T) {
	src := `
val x: String? = null
val result = x == null
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "equality_expression")
	if idx == 0 {
		t.Fatal("expected to find equality_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "Boolean" {
		t.Errorf("expected Boolean, got %v", got)
	}
}

func TestBinaryExpr_Conjunction(t *testing.T) {
	src := `val result = true && false`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "conjunction_expression")
	if idx == 0 {
		t.Fatal("expected to find conjunction_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "Boolean" {
		t.Errorf("expected Boolean, got %v", got)
	}
}

func TestBinaryExpr_Disjunction(t *testing.T) {
	src := `val result = true || false`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "disjunction_expression")
	if idx == 0 {
		t.Fatal("expected to find disjunction_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "Boolean" {
		t.Errorf("expected Boolean, got %v", got)
	}
}

func TestBinaryExpr_PrefixNot(t *testing.T) {
	src := `val result = !true`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "prefix_expression")
	if idx == 0 {
		t.Fatal("expected to find prefix_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "Boolean" {
		t.Errorf("expected Boolean, got %v", got)
	}
}

func TestBinaryExpr_PrefixNegation(t *testing.T) {
	src := `val result = -42`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfType(file, "prefix_expression")
	if idx == 0 {
		t.Fatal("expected to find prefix_expression node")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil || got.Name != "Int" {
		t.Errorf("expected Int, got %v", got)
	}
}
