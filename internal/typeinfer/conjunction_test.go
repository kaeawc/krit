package typeinfer

import (
	"testing"
)

func TestSmartCast_ConjunctionDoubleNullCheck(t *testing.T) {
	src := `
fun example(x: String?, y: String?) {
    if (x != null && y != null) {
        val a = x.length
        val b = y.length
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.RootScope.Children) == 0 {
		t.Fatal("expected at least one child scope (function scope)")
	}
	funcScope := fi.RootScope.Children[0]

	// Find the child scope that has SmartCasts for both x and y
	var ifBodyScope *ScopeTable
	for _, child := range funcScope.Children {
		if child.SmartCasts["x"] && child.SmartCasts["y"] {
			ifBodyScope = child
			break
		}
	}
	if ifBodyScope == nil {
		t.Fatal("expected a child scope with smart casts for both x and y")
	}

	// Verify both x and y have smart casts
	if !ifBodyScope.SmartCasts["x"] {
		t.Error("expected x to have smart cast (non-null) in if-body scope")
	}
	if !ifBodyScope.SmartCasts["y"] {
		t.Error("expected y to have smart cast (non-null) in if-body scope")
	}
}

func TestSmartCast_ConjunctionNullCheckAndIsCheck(t *testing.T) {
	src := `
fun example(x: Any?) {
    if (x != null && x is String) {
        val len = x.length
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.RootScope.Children) == 0 {
		t.Fatal("expected at least one child scope (function scope)")
	}
	funcScope := fi.RootScope.Children[0]

	// Find the child scope that has both a null smart cast and a type smart cast for x
	var ifBodyScope *ScopeTable
	for _, child := range funcScope.Children {
		hasNull := child.SmartCasts["x"]
		_, hasType := child.SmartCastTypes["x"]
		if hasNull && hasType {
			ifBodyScope = child
			break
		}
	}
	if ifBodyScope == nil {
		t.Fatal("expected a child scope with both null smart cast and type smart cast for x")
	}

	// x should resolve to String in the if-body scope
	got := ifBodyScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in if-body scope")
	}
	if got.Name != "String" {
		t.Errorf("expected x to be String inside if body, got %q", got.Name)
	}
}

func TestSmartCast_SingleConditionNoConjunction(t *testing.T) {
	// Regression test: single null check should still work through existing path
	src := `
fun example(x: String?) {
    if (x != null) {
        val a = x.length
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.RootScope.Children) == 0 {
		t.Fatal("expected at least one child scope (function scope)")
	}
	funcScope := fi.RootScope.Children[0]

	// Find the child scope that has SmartCast for x
	var ifBodyScope *ScopeTable
	for _, child := range funcScope.Children {
		if child.SmartCasts["x"] {
			ifBodyScope = child
			break
		}
	}
	if ifBodyScope == nil {
		t.Fatal("expected a child scope with smart cast for x")
	}

	if !ifBodyScope.SmartCasts["x"] {
		t.Error("expected x to have smart cast (non-null) in if-body scope")
	}
}
