package rules_test

import (
	"sort"
	"testing"
)

func countRule(t *testing.T, rule, code string) int {
	t.Helper()
	return len(runRuleByName(t, rule, code))
}

func TestEndOfSentenceFormat_UsesWholeDefaultSection(t *testing.T) {
	code := `package test

/**
 * Summary line
 *
 * Body paragraph.
 */
fun a() {}

/**
 * Summary line
 * @param x value
 */
fun b(x: Int) {}
`
	if got := countRule(t, "EndOfSentenceFormat", code); got != 1 {
		t.Fatalf("want 1 finding (only b, no sentence end before the tag), got %d", got)
	}
}

func TestEndOfSentenceFormat_AcceptsTrailingURL(t *testing.T) {
	code := `package test

/** See https://example.com/x */
fun a() {}

/** See https://example.com/x for more */
fun b() {}

/** foo://bar */
fun c() {}
`
	if got := countRule(t, "EndOfSentenceFormat", code); got != 2 {
		t.Fatalf("want 2 findings (b and c), got %d", got)
	}
}

func TestEndOfSentenceFormat_SkipsOrphanKDoc(t *testing.T) {
	code := `package test

fun a() {
    /** orphan without period */
    println(1)
}

class B {
    fun f() {}
    /** trailing orphan */
}
`
	if got := countRule(t, "EndOfSentenceFormat", code); got != 0 {
		t.Fatalf("want no findings for KDocs that document nothing, got %d", got)
	}
}

func TestDeprecatedBlockTag_RequiresLineStart(t *testing.T) {
	code := `package test

/** Sum. @deprecated inline */
fun a() {}

/**
 * Sum.
 * @deprecated use c
 */
fun b() {}
`
	if got := countRule(t, "DeprecatedBlockTag", code); got != 1 {
		t.Fatalf("want 1 finding (only the block tag), got %d", got)
	}
}

func TestOutdatedDocumentation_SecondaryConstructor(t *testing.T) {
	code := `package test

class A(val x: Int) {
    /**
     * Doc.
     * @param p value
     * @param q value
     */
    constructor(p: Int) : this(p)

    /**
     * Doc.
     * @param p value
     */
    constructor(p: Int, q: Int, r: Int) : this(p)

    /**
     * Doc.
     * @param p value
     */
    constructor(p: String) : this(1)
}
`
	findings := runRuleByName(t, "OutdatedDocumentation", code)
	if len(findings) != 2 || findings[0].Line != 4 || findings[1].Line != 11 {
		t.Fatalf("want findings at the KDoc start of the first two constructors, got %v", findings)
	}
}

func TestOutdatedDocumentation_PropertyTags(t *testing.T) {
	code := `package test

/**
 * Doc.
 * @property a value
 * @property b value
 */
internal class Internal internal constructor(a: Int, val b: Int)

/**
 * Doc.
 * @property a value
 */
class Restricted private constructor(val a: Int, b: Int)

/**
 * Doc.
 * @property a value
 * @param b value
 */
class Ok(val a: Int, b: Int)

/**
 * Doc.
 * @property zz value
 */
class PrivateProp(private val zz: Int)

/**
 * Doc.
 * @property b value
 */
fun propertyTagOnFunction(b: Int) {}
`
	findings := runRuleByName(t, "OutdatedDocumentation", code)
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	want := []int{8, 27, 33}
	if len(lines) != len(want) {
		t.Fatalf("want findings at %v, got %v", want, lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("want findings at %v, got %v", want, lines)
		}
	}
}

func TestUndocumentedPublicProperty_ParamTagDocumentsProperty(t *testing.T) {
	code := `package test

/**
 * Doc.
 * @param a value
 * @property b value
 */
class A(val a: Int, val b: Int, val c: Int)
`
	findings := runRuleByName(t, "UndocumentedPublicProperty", code)
	if len(findings) != 1 {
		t.Fatalf("want 1 finding (only c), got %d", len(findings))
	}
}

func TestUndocumentedPublicFunction_SkipsAnonymousObjectMembers(t *testing.T) {
	code := `package test

/** Doc. */
val anon = object { fun inside() {} }
`
	if got := countRule(t, "UndocumentedPublicFunction", code); got != 0 {
		t.Fatalf("want no findings for anonymous object members, got %d", got)
	}
}

func TestEndOfSentenceFormat_FixGoesAfterLastTextLine(t *testing.T) {
	code := "package test\n\n/**\n * Intro to a list:\n * - one\n * @param x value\n */\nfun a(x: Int) {}\n"
	findings := runRuleByName(t, "EndOfSentenceFormat", code)
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("want one finding with a fix, got %+v", findings)
	}
	fix := findings[0].Fix
	got := code[:fix.StartByte] + fix.Replacement + code[fix.EndByte:]
	want := "package test\n\n/**\n * Intro to a list:\n * - one.\n * @param x value\n */\nfun a(x: Int) {}\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
