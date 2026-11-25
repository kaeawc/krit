package typeinfer

import (
	"strings"
	"testing"
)

// smartCastNameAt returns the type name the scope table gives name at the first occurrence of marker in src.
func smartCastNameAt(t *testing.T, src, marker, name string) string {
	t.Helper()
	offset := strings.Index(src, marker)
	if offset < 0 {
		t.Fatalf("marker %q not in source", marker)
	}
	fi := IndexFileParallel(parseTestFile(t, src))
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}
	scope := fi.RootScope.FindScopeAtOffset(uint32(offset))
	if scope == nil {
		t.Fatalf("no scope at marker %q", marker)
	}
	got := scope.LookupAt(name, uint32(offset))
	if got == nil {
		t.Fatalf("%s not visible at marker %q", name, marker)
	}
	return got.Name
}

func TestSmartCast_IfIsCheckNarrowsThenBranchOnly(t *testing.T) {
	src := `
fun example(x: Any) {
    if (x is String) {
        x.hashCode() // then
    } else {
        x.hashCode() // else
    }
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // then", "x"); got != "String" {
		t.Errorf("then-branch: x = %q, want String", got)
	}
	if got := smartCastNameAt(t, src, "x.hashCode() // else", "x"); got != "Any" {
		t.Errorf("else-branch: x = %q, want Any (x is not String there)", got)
	}
}

func TestSmartCast_IfIsCheckExpressionElseNotNarrowed(t *testing.T) {
	src := `
fun example(x: Any): Int =
    if (x is String) x.length
    else x.hashCode()
`
	if got := smartCastNameAt(t, src, "x.length", "x"); got != "String" {
		t.Errorf("then-branch: x = %q, want String", got)
	}
	if got := smartCastNameAt(t, src, "x.hashCode()", "x"); got != "Any" {
		t.Errorf("else-branch: x = %q, want Any", got)
	}
}

func TestSmartCast_IfNegatedIsCheckNarrowsElseBranch(t *testing.T) {
	src := `
fun example(x: Any) {
    if (x !is String) {
        x.hashCode() // then
    } else {
        x.hashCode() // else
    }
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // then", "x"); got != "Any" {
		t.Errorf("then-branch: x = %q, want Any", got)
	}
	if got := smartCastNameAt(t, src, "x.hashCode() // else", "x"); got != "String" {
		t.Errorf("else-branch: x = %q, want String", got)
	}
}

func TestSmartCast_IfConjunctionNarrowsThenBranchOnly(t *testing.T) {
	src := `
fun example(x: Any, flag: Boolean) {
    if (flag && x is String) {
        x.hashCode() // then
    } else {
        x.hashCode() // else
    }
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // then", "x"); got != "String" {
		t.Errorf("then-branch: x = %q, want String", got)
	}
	if got := smartCastNameAt(t, src, "x.hashCode() // else", "x"); got != "Any" {
		t.Errorf("else-branch: x = %q, want Any", got)
	}
}

func TestSmartCast_IfIsCheckUnderDisjunctionOrNegationDoesNotNarrow(t *testing.T) {
	for _, cond := range []string{"flag || x is String", "!(x is String)", "(x is String) == flag"} {
		src := "fun example(x: Any, flag: Boolean) {\n    if (" + cond + ") {\n        x.hashCode() // then\n    }\n}\n"
		if got := smartCastNameAt(t, src, "x.hashCode() // then", "x"); got != "Any" {
			t.Errorf("if (%s): x = %q, want Any", cond, got)
		}
	}
	src := `
fun example(x: Any) {
    if ((x is String)) {
        x.hashCode() // then
    }
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // then", "x"); got != "String" {
		t.Errorf("parenthesized check: x = %q, want String", got)
	}
}

func TestSmartCast_IfNotNullCheckNarrowsThenBranchOnly(t *testing.T) {
	src := `
fun example(x: String?) {
    if (x != null) {
        x.length // then
    } else {
        x.hashCode() // else
    }
}
`
	offsetThen := "x.length // then"
	offsetElse := "x.hashCode() // else"
	fi := IndexFileParallel(parseTestFile(t, src))
	for _, tc := range []struct {
		marker       string
		wantNullable bool
	}{{offsetThen, false}, {offsetElse, true}} {
		off := uint32(strings.Index(src, tc.marker))
		got := fi.RootScope.FindScopeAtOffset(off).LookupAt("x", off)
		if got == nil || got.IsNullable() != tc.wantNullable {
			t.Errorf("%s: x = %+v, want nullable=%v", tc.marker, got, tc.wantNullable)
		}
	}
}

func TestSmartCast_WhenEntryWithSeveralTypeConditionsDoesNotNarrow(t *testing.T) {
	src := `
fun example(x: Any) {
    when (x) {
        is String, is Int -> x.hashCode() // both
        is Long -> x.hashCode() // long
        else -> x.hashCode() // else
    }
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // both", "x"); got != "Any" {
		t.Errorf("`is String, is Int` entry: x = %q, want Any", got)
	}
	if got := smartCastNameAt(t, src, "x.hashCode() // long", "x"); got != "Long" {
		t.Errorf("`is Long` entry: x = %q, want Long", got)
	}
	if got := smartCastNameAt(t, src, "x.hashCode() // else", "x"); got != "Any" {
		t.Errorf("else entry: x = %q, want Any", got)
	}
}

// smartCastNullableAt reports whether the scope table gives name a nullable type at the first occurrence of marker.
func smartCastNullableAt(t *testing.T, src, marker, name string) bool {
	t.Helper()
	offset := strings.Index(src, marker)
	if offset < 0 {
		t.Fatalf("marker %q not in source", marker)
	}
	fi := IndexFileParallel(parseTestFile(t, src))
	got := fi.RootScope.FindScopeAtOffset(uint32(offset)).LookupAt(name, uint32(offset))
	if got == nil {
		t.Fatalf("%s not visible at marker %q", name, marker)
	}
	return got.IsNullable()
}

// Base pin: a branch that is not narrowed keeps its own scope, so an elvis early exit inside it does not make
// the variable non-null after the if.
func TestSmartCast_UnnarrowedBranchFactsStayInBranch(t *testing.T) {
	for _, src := range []string{`
fun example(a: String?, b: String?) {
    if (a != null) {
        a.length
    } else {
        val v = b ?: return
    }
    b?.length // after
}
`, `
fun example(a: Any, b: String?) {
    if (a is String) {
        a.length
    } else {
        val v = b ?: return
    }
    b?.length // after
}
`, `
fun example(a: String?, flag: Boolean, b: String?) {
    if (flag && a != null) {
        a.length
    } else {
        val v = b ?: return
    }
    b?.length // after
}
`} {
		if !smartCastNullableAt(t, src, "b?.length // after", "b") {
			t.Errorf("b must stay nullable after the if:\n%s", src)
		}
	}
}

// Base pin: a local declared in a branch that is not narrowed does not shadow a later same-named local.
func TestSmartCast_UnnarrowedBranchLocalsStayInBranch(t *testing.T) {
	src := `
fun example(a: String?) {
    if (a != null) {
        a.length
    } else {
        val tmp = 1
    }
    val tmp = "s"
    tmp.length // after
}
`
	if got := smartCastNameAt(t, src, "tmp.length // after", "tmp"); got != "String" {
		t.Errorf("tmp after the if = %q, want String", got)
	}
}

// Base pin: `is` narrows the then-branch of a conjunction with three or more operands, or a parenthesized one.
func TestSmartCast_IsCheckInNestedConjunctionNarrows(t *testing.T) {
	for _, cond := range []string{"x is String && a && b", "(x is String) && b", "a && x is String && b", "a && (b && x is String)"} {
		src := "fun example(x: Any, a: Boolean, b: Boolean) {\n    if (" + cond + ") {\n        x.hashCode() // then\n    }\n}\n"
		if got := smartCastNameAt(t, src, "x.hashCode() // then", "x"); got != "String" {
			t.Errorf("if (%s): x = %q, want String", cond, got)
		}
	}
}

// Base pin: `x !is T || ...` with an early exit leaves x a T after the if.
func TestSmartCast_NegatedIsCheckInDisjunctionEarlyReturnNarrows(t *testing.T) {
	for _, cond := range []string{"x !is String || flag", "flag || x !is String", "(x !is String) || flag"} {
		src := "fun example(x: Any, flag: Boolean) {\n    if (" + cond + ") return\n    x.hashCode() // after\n}\n"
		if got := smartCastNameAt(t, src, "x.hashCode() // after", "x"); got != "String" {
			t.Errorf("if (%s) return: x = %q, want String", cond, got)
		}
	}
	src := `
fun example(x: Any?) {
    if (x == null || x !is String) return
    x.hashCode() // after
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // after", "x"); got != "String" {
		t.Errorf("if (x == null || x !is String) return: x = %q, want String", got)
	}
}

// `x !is T || ...` also leaves x a T in the else-branch, and not in the then-branch.
func TestSmartCast_NegatedIsCheckInDisjunctionNarrowsElseBranch(t *testing.T) {
	src := `
fun example(x: Any, flag: Boolean) {
    if (x !is String || flag) {
        x.hashCode() // then
    } else {
        x.hashCode() // else
    }
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // then", "x"); got != "Any" {
		t.Errorf("then-branch: x = %q, want Any", got)
	}
	if got := smartCastNameAt(t, src, "x.hashCode() // else", "x"); got != "String" {
		t.Errorf("else-branch: x = %q, want String", got)
	}
}

// `!(x is T)` behaves like `x !is T`: the else-branch (base pin) and the code after an early exit see a T,
// the then-branch does not.
func TestSmartCast_NegatedPrefixIsCheckNarrowsElseBranch(t *testing.T) {
	src := `
fun example(x: Any) {
    if (!(x is String)) {
        x.hashCode() // then
    } else {
        x.hashCode() // else
    }
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // else", "x"); got != "String" {
		t.Errorf("else-branch: x = %q, want String", got)
	}
	if got := smartCastNameAt(t, src, "x.hashCode() // then", "x"); got != "Any" {
		t.Errorf("then-branch: x = %q, want Any", got)
	}
	early := `
fun example(x: Any) {
    if (!(x is String)) return
    x.hashCode() // after
}
`
	if got := smartCastNameAt(t, early, "x.hashCode() // after", "x"); got != "String" {
		t.Errorf("after `if (!(x is String)) return`: x = %q, want String", got)
	}
}

// `x !is T && y` being false leaves x possibly "not T", so neither the else-branch nor the code after an
// early exit narrows x.
func TestSmartCast_NegatedIsCheckInConjunctionDoesNotNarrow(t *testing.T) {
	early := `
fun example(x: Any, flag: Boolean) {
    if (x !is String && flag) return
    x.hashCode() // after
}
`
	if got := smartCastNameAt(t, early, "x.hashCode() // after", "x"); got != "Any" {
		t.Errorf("after `if (x !is String && flag) return`: x = %q, want Any", got)
	}
	branches := `
fun example(x: Any, flag: Boolean) {
    if (x !is String && flag) {
        x.hashCode() // then
    } else {
        x.hashCode() // else
    }
}
`
	if got := smartCastNameAt(t, branches, "x.hashCode() // else", "x"); got != "Any" {
		t.Errorf("else-branch: x = %q, want Any", got)
	}
}

// Base pin: an if whose condition narrows nothing still keeps its branch locals inside the branch.
func TestSmartCast_FallbackBranchLocalsStayInBranch(t *testing.T) {
	for _, cond := range []string{"flag || a is String", "!(a is String) == flag"} {
		src := "fun example(a: Any, flag: Boolean) {\n    if (" + cond + ") {\n        val tmp = 1\n    } else {\n        val tmp = 2\n    }\n    val tmp = \"s\"\n    tmp.length // after\n}\n"
		if got := smartCastNameAt(t, src, "tmp.length // after", "tmp"); got != "String" {
			t.Errorf("if (%s): tmp after the if = %q, want String", cond, got)
		}
	}
}

// Base pin: `!(x is T)` narrows after the if only when the then-body always exits, not when it holds a
// conditional return.
func TestSmartCast_NegatedPrefixGuardNeedsUnconditionalExit(t *testing.T) {
	src := `
fun example(x: Any, flag: Boolean) {
    if (!(x is String)) {
        if (flag) return
        println()
    }
    x.hashCode() // after
}
`
	if got := smartCastNameAt(t, src, "x.hashCode() // after", "x"); got != "Any" {
		t.Errorf("after a conditional exit: x = %q, want Any", got)
	}
	exits := `
fun example(x: Any) {
    if (!(x is String)) {
        println()
        return
    }
    x.hashCode() // after
}
`
	if got := smartCastNameAt(t, exits, "x.hashCode() // after", "x"); got != "String" {
		t.Errorf("after an unconditional exit: x = %q, want String", got)
	}
	localError := `fun error() {}

fun example(x: Any) {
    if (!(x is String)) error()
    x.hashCode() // after
}
`
	if got := smartCastNameAt(t, localError, "x.hashCode() // after", "x"); got != "Any" {
		t.Errorf("after a local error() that returns: x = %q, want Any", got)
	}
}
