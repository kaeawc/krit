package rules_test

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
)

// The expected findings below are what the Android Lint 32.4.1 CLI reports for the same sources (a project with
// kotlin-stdlib on the classpath), as "line:col message".

func TestUnnecessaryArrayInit_MatchesLint(t *testing.T) {
	src := `package p

const val ZERO = 0

fun arrays() {
    val a = IntArray(3) { 0 }
    val b = LongArray(3) { 0L }
    val c = FloatArray(3) { 0f }
    val d = DoubleArray(3) { 0.0 }
    val e = BooleanArray(3) { false }
    val f = CharArray(3) { '\u0000' }
    val g = ShortArray(3) { 0 }
    val i = IntArray(3) { it }
    val j = IntArray(3) { 1 - 1 }
    val k = IntArray(3) { -0 }
    val l = IntArray(3) { i -> 0 }
    val m = IntArray(3, { 0 })
    val n = UIntArray(3) { 0u }
    val o = Array(3) { 0 }
    val p = IntArray(3) { return@IntArray 0 }
    val q = DoubleArray(3) { 0.0 + 0 }
    val r = IntArray(3) { 0x0 }
    val s = IntArray(3) { ZERO }
    val t = IntArray(3) { (0) }
    val u = IntArray(3) { println(); 0 }
    val v = IntArray(3) { -1 + 1 }
}
`
	got := appCompatSourceFindings(t, "UnnecessaryArrayInit", "src/main/java/p/A.kt", src, librarymodel.ProjectProfile{})
	msg := func(lambda string) string {
		return "This initialization lambda (`" + lambda + "`) is unnecessary and is less efficient"
	}
	want := []string{
		"12:27 " + msg("{ 0 }"),
		"14:25 " + msg("{ 1 - 1 }"),
		"15:25 " + msg("{ -0 }"),
		"16:25 " + msg("{ i -> 0 }"),
		"17:25 " + msg("{ 0 }"),
		"20:25 " + msg("{ return@IntArray 0 }"),
		"21:28 " + msg("{ 0.0 + 0 }"),
		"22:25 " + msg("{ 0x0 }"),
		"23:25 " + msg("{ ZERO }"),
		"24:25 " + msg("{ (0) }"),
		"26:25 " + msg("{ -1 + 1 }"),
		"6:25 " + msg("{ 0 }"),
		"7:26 " + msg("{ 0L }"),
		"8:27 " + msg("{ 0f }"),
		"9:28 " + msg("{ 0.0 }"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings:\n got %q\nwant %q", got, want)
	}
}

func TestUnnecessaryArrayInit_LocalLookalikes(t *testing.T) {
	src := `package p

import com.example.IntArray

class LongArray(size: Int, init: (Int) -> Long)

fun arrays() {
    val a = IntArray(3) { 0 }
    val b = LongArray(3) { 0L }
    val c = kotlin.LongArray(3) { 0L }
}
`
	got := appCompatSourceFindings(t, "UnnecessaryArrayInit", "src/main/java/p/A.kt", src, librarymodel.ProjectProfile{})
	// Lint agrees when com.example.IntArray exists; an import that does not resolve leaves Lint reporting it.
	want := []string{"10:33 This initialization lambda (`{ 0L }`) is unnecessary and is less efficient"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings:\n got %q\nwant %q", got, want)
	}
}

const expensiveAssertionSample = `package p

class B {
    var c = 0
    fun a6() { assert(Thing() == null) }
    fun a7() { assert(Other(1) == null) }
    fun a8() { assert(!(c++ > 0)) }
    fun a10() { assert(isOk() ?: (c++ > 0)) }
    fun a11() { assert(depth1()) }
    fun a12() { assert(setFoo(1)) }
    fun a13() { assert(run { c++ > 0 }) }
    fun a14() { assert(when { c++ > 0 -> true; else -> false }) }
    fun a15() { assert(if (c++ > 0) true else false) }
    fun a16() { assert(local()) }
    fun local(): Boolean { var y = 0; y++; return true }
    fun depth1(): Boolean = depth2()
    fun depth2(): Boolean = depth3()
    fun depth3(): Boolean { c++; return true }
    fun setFoo(x: Int): Boolean = true
    fun isOk(): Boolean? = true
    fun e1() { assert(Other(1).ok) }
    fun e3() { assert(listOf(1) == null) }
    fun e4() { assert(contains(1)) }
    fun contains(x: Int) = true
    fun e5() { assert(c in 0..10) }
    fun e6() { assert(c > 0) { "msg ${c}" } }
    fun e7() { assert(-c + 1 > 0) }
    fun e9() { assert(c > 0 && c > 1 && c > 2 && c > 3 && c > 4) }
    fun e10() { assert(isEmpty()) }
    fun e11() { assert(this.isOk() == true) }
    fun e12() { assert("${depth3()}" == "") }
    fun e13() { if (javaClass.desiredAssertionStatus()) { assert(local()) } }
    fun e14() { assert(fromOtherFile()) }
}
class Thing
class Other(val x: Int) { val ok = true }
fun isEmpty() = true
fun t() { assert(maxOf(1, 2) > 0) }
`

func TestExpensiveAssertion_MatchesLint(t *testing.T) {
	got := appCompatSourceFindings(t, "ExpensiveAssertion", "src/main/java/p/B.kt", expensiveAssertionSample, librarymodel.ProjectProfile{})
	const inClass = "Kotlin assertion arguments are always evaluated, even when assertions are off. Consider surrounding assertion with `if (javaClass.desiredAssertionStatus()) { assert(...) }`"
	// e14 calls a function the file does not declare. Lint reads its body when it is in another source file and
	// counts it as expensive when it does not resolve; krit cannot see other files here, so it abstains.
	want := []string{
		"11:24 " + inClass,
		"12:24 " + inClass,
		"13:24 " + inClass,
		"14:24 " + inClass,
		"22:23 " + inClass,
		"28:23 " + inClass,
		"31:24 " + inClass,
		"38:18 Kotlin assertion arguments are always evaluated, even when assertions are off",
		"5:23 " + inClass,
		"6:23 " + inClass,
		"8:24 " + inClass,
		"9:24 " + inClass,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings:\n got %q\nwant %q", got, want)
	}
}

func TestAssertionSideEffect_Kotlin_MatchesLint(t *testing.T) {
	got := appCompatSourceFindings(t, "AssertionSideEffect", "src/main/java/p/B.kt", expensiveAssertionSample+`
class C(val list: MutableList<Int>, val file: java.io.File) {
    var c = 0
    fun b1() { assert(ArrayList<Int>().add(1)) }
    fun b2() { assert(HashMap<String, Int>().put("a", 1) == null) }
    fun b3() { assert(mutableListOf(1).add(1)) }
    fun b4() { assert(StringBuilder().setLength(0) == Unit) }
    fun b5(s: MutableSet<Int>) { assert(s.remove(1)) }
    fun b6() { assert(!isOk() && c++ > 0) }
    fun b7() { assert(file.mkdirs()) }
    fun b8() { assert(list.add(1)) }
    fun b9() { var x = 0; assert(x++ == 0) }
    fun b10() { assert(c-- > 0) }
    fun isOk(): Boolean = true
}
`, librarymodel.ProjectProfile{})
	const p = "Assertion condition has a side effect: "
	want := []string{
		"13:28 " + p + "c++",
		"31:27 " + p + "c++",
		"42:23 " + p + "add(1)",
		"43:23 " + p + "put(\"a\", 1)",
		"45:23 " + p + "setLength(0)",
		"47:34 " + p + "c++",
		"48:23 " + p + "mkdirs()",
		"50:34 " + p + "x++",
		"51:24 " + p + "c--",
		"9:24 " + p + "c++",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings:\n got %q\nwant %q", got, want)
	}
}

func TestAssertionSideEffect_Java(t *testing.T) {
	src := `package p;
import java.util.List;
class J {
    int c;
    void a(List<Integer> list, java.io.File f) {
        assert c++ > 0;
        assert list.add(1);
        assert f.mkdirs();
        assert (c = 2) > 0 : "message";
        int x = 0;
        assert x++ == 0;
        assert sideEffect();
        assert !(c > 0) && --c > 0;
        // Not assertions: increments outside an assert statement.
        c++;
        assertTrue(c++ > 0);
    }
    boolean sideEffect() { c++; return true; }
    static void assertTrue(boolean b) {}
}
`
	got := appCompatSourceFindings(t, "AssertionSideEffect", "src/main/java/p/J.java", src, librarymodel.ProjectProfile{})
	// Lint does not follow Java calls: list.add, f.mkdirs and sideEffect() are not reported.
	want := []string{
		"11:16 Assertion condition has a side effect: x++",
		"13:28 Assertion condition has a side effect: --c",
		"6:16 Assertion condition has a side effect: c++",
		"9:17 Assertion condition has a side effect: c = 2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings:\n got %q\nwant %q", got, want)
	}
}

func TestAssertionRules_ShadowedAssert(t *testing.T) {
	src := `package p

import com.example.assert

class A {
    var c = 0
    fun f() { assert(c++ > 0) }
}
`
	for _, rule := range []string{"AssertionSideEffect", "ExpensiveAssertion"} {
		if got := appCompatSourceFindings(t, rule, "src/main/java/p/A.kt", src, librarymodel.ProjectProfile{}); len(got) != 0 {
			t.Errorf("%s: an imported assert is not kotlin.assert, got %q", rule, got)
		}
	}
	member := `package p

class A {
    var c = 0
    fun assert(b: Boolean) = Unit
    fun f() { assert(c++ > 0) }
}

class B {
    var c = 0
    fun g() { assert(c++ > 0) }
}
`
	got := appCompatSourceFindings(t, "AssertionSideEffect", "src/main/java/p/A.kt", member, librarymodel.ProjectProfile{})
	if want := []string{"11:22 Assertion condition has a side effect: c++"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("a member assert hides kotlin.assert only in its class:\n got %q\nwant %q", got, want)
	}
}

// kotlinx.coroutines declares `internal inline fun assert(value: () -> Boolean)` in another file and calls it as
// `assert { cond }`. kotlin.assert takes its condition in parentheses, so a lambda-only call is never kotlin.assert.
func TestAssertionRules_LambdaOnlyAssertIsNotKotlinAssert(t *testing.T) {
	src := `package p

class E {
    var c = 0
    fun g() { assert { c++ > 0 && compute() } }
    fun compute(): Boolean { println(); return true }
}
`
	for _, rule := range []string{"AssertionSideEffect", "ExpensiveAssertion"} {
		if got := appCompatSourceFindings(t, rule, "src/main/java/p/E.kt", src, librarymodel.ProjectProfile{}); len(got) != 0 {
			t.Errorf("%s: assert { ... } is not kotlin.assert, got %q", rule, got)
		}
	}
}
