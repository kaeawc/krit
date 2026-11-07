package rules_test

import (
	"strings"
	"testing"
)

const whenNestedSealedDecls = `
package test

sealed interface Node
data class Leaf(val x: Int) : Node
sealed interface Branch : Node
data class Left(val x: Int) : Branch
data class Right(val x: Int) : Branch
`

func TestElseCaseInsteadOfExhaustiveWhen_NestedSealedSubtypeNotCoveredIsIgnored(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenNestedSealedDecls+`
fun a(n: Node): Int = when (n) {
  is Leaf -> 1
  else -> 0
}
fun b(n: Node): Int = when (n) {
  is Leaf -> 1
  is Left -> 2
  else -> 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("an uncovered nested sealed subtype keeps else needed, got %d", len(findings))
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_GuardedBranchDoesNotCover(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenNestedSealedDecls+`
fun a(n: Node): Int = when (n) {
  is Leaf if n.x > 0 -> 1
  is Branch -> 2
  else -> 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("a guarded branch only covers some instances, got %d", len(findings))
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_QualifiedTestOfOtherClassDoesNotCover(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", `
package test

sealed class Shape
class Circle : Shape()
class Square : Shape()

fun a(shape: Shape): Int = when (shape) {
  is Circle -> 1
  is Other.Square -> 2
  else -> 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("a qualified test of a different class must not cover Square, got %d", len(findings))
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_ReportsFullNestedCoverageWithoutFix(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenNestedSealedDecls+`
fun a(n: Node): Int = when (n) {
  is Leaf -> 1
  is Left -> 2
  is Right -> 3
  else -> 0
}
fun b(n: Node): Int = when (n) {
  is Leaf -> 1
  is Branch -> 2
  else -> 0
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both fully covered whens to be reported, got %d", len(findings))
	}
	for _, f := range findings {
		if f.Fix != nil || !strings.Contains(f.Message, "should not contain an `else` case") {
			t.Fatalf("expected a message-only finding, got %+v", f)
		}
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_QualifiedNestedVariantsStillCover(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", `
package test

sealed class Shape {
  class Circle : Shape()
  class Square : Shape()
}

fun a(shape: Shape): Int = when (shape) {
  is Shape.Circle -> 1
  is Shape.Square -> 2
  else -> 0
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected the fully covered qualified when to be reported, got %d", len(findings))
	}
}

const whenSameNamedNestedDecls = `
package test

import test.Entry.PaidItem
import test.Entry.OpenItem

sealed class Entry {
  data class PaidItem(val ref: String) : Entry()
  data class OpenItem(val seq: Long) : Entry()
  sealed class Draft : Entry() {
    data class PaidItem(val ref: String) : Draft()
    data class OpenItem(val seq: Long) : Draft()
  }
}
`

func TestElseCaseInsteadOfExhaustiveWhen_SameNamedLeavesOfNestedSealedDoNotCoverIt(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenSameNamedNestedDecls+`
fun a(e: Entry): Int = when (e) {
  is PaidItem -> 1
  is OpenItem -> 2
  else -> 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("the nested sealed Draft is not covered by tests of the outer same-named classes, got %d", len(findings))
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_SameNamedLeavesCoveredByQualifiedTests(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenSameNamedNestedDecls+`
fun a(e: Entry): Int = when (e) {
  is PaidItem -> 1
  is OpenItem -> 2
  is Entry.Draft.PaidItem -> 3
  is Entry.Draft.OpenItem -> 4
  else -> 0
}
fun b(e: Entry): Int = when (e) {
  is PaidItem -> 1
  is OpenItem -> 2
  is Entry.Draft -> 3
  else -> 0
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both fully covered whens to be reported, got %d", len(findings))
	}
}

const whenObjectVariantDecls = `
package test

sealed interface Result<out T> {
  data class Loaded<T>(val value: T) : Result<T>
  data object Loading : Result<Nothing>
}
`

func TestElseCaseInsteadOfExhaustiveWhen_ObjectVariantOnlyHandledByElseIsNeeded(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenObjectVariantDecls+`
fun <T> Result<T>.getOrNull(): T? = when (this) {
  is Result.Loaded -> value
  else -> null
}
`)
	if len(findings) != 0 {
		t.Fatalf("else is the only handler of the Loading object, got %d", len(findings))
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_ObjectVariantsCoveredByIsAndEquality(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenObjectVariantDecls+`
fun a(r: Result<Int>): Int = when (r) {
  is Result.Loaded -> 1
  is Result.Loading -> 2
  else -> 0
}
fun b(r: Result<Int>): Int = when (r) {
  is Result.Loaded -> 1
  Result.Loading -> 2
  else -> 0
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both fully covered whens to be reported, got %d", len(findings))
	}
}

func TestNoElseInWhenSealed_ObjectVariantMatchedByEqualityIsCovered(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NoElseInWhenSealed", whenObjectVariantDecls+`
fun a(r: Result<Int>) {
  when (r) {
    is Result.Loaded -> println(1)
    Result.Loading -> println(2)
  }
}
fun b(r: Result<Int>) {
  when (r) {
    is Result.Loaded -> println(1)
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("only the when missing the Loading object should be reported, got %d", len(findings))
	}
}

const whenEnumDecls = `
package test
enum class Color { Red, Green }
`

func TestElseCaseInsteadOfExhaustiveWhen_EnumAndBooleanSubjects(t *testing.T) {
	flagged := map[string]string{
		"enum":    "fun f(c: Color) = when (c) { Color.Red -> 1; Color.Green -> 2; else -> 3 }",
		"boolean": "fun f(b: Boolean) = when (b) { true -> 1; false -> 2; else -> 3 }",
	}
	for name, src := range flagged {
		got := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenEnumDecls+src+"\n")
		if len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"partial enum":    "fun f(c: Color) = when (c) { Color.Red -> 1; else -> 3 }",
		"partial boolean": "fun f(b: Boolean) = when (b) { true -> 1; else -> 3 }",
		"int subject":     "fun f(i: Int) = when (i) { 1 -> 1; else -> 3 }",
	}
	for name, src := range clean {
		if got := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", whenEnumDecls+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
}

const whenGuardDecls = `
package test

sealed interface S
class A : S
class B : S
enum class E { X, Y }
`

func TestWhenExhaustivenessRules_GuardedBranchesCoverNothing(t *testing.T) {
	missing := whenGuardDecls + `
fun a(s: S, flag: Boolean) = when (s) {
  is A if flag -> 1
  is B -> 2
}
fun b(s: S, flag: Boolean) {
  when (s) {
    is A if flag -> println()
    is B -> println()
  }
}
`
	if got := runRuleByNameWithResolver(t, "NoElseInWhenSealed", missing); len(got) != 2 {
		t.Errorf("NoElseInWhenSealed: a guarded branch leaves A uncovered, want 2 findings, got %d", len(got))
	}
	if got := runRuleByNameWithResolver(t, "NonExhaustiveWhen", missing); len(got) != 1 {
		t.Errorf("NonExhaustiveWhen: the expression leaves A uncovered, want 1 finding, got %d", len(got))
	}
	covered := whenGuardDecls + `
fun a(s: S, flag: Boolean) = when (s) {
  is A if flag -> 1
  is A -> 2
  is B -> 3
}
`
	if got := runRuleByNameWithResolver(t, "NoElseInWhenSealed", covered); len(got) != 0 {
		t.Errorf("an unguarded branch covers A, want no findings, got %d", len(got))
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_EnumGuardsAreCounted(t *testing.T) {
	guardedOnly := whenGuardDecls + `
fun a(e: E, flag: Boolean) = when (e) {
  E.X if flag -> 1
  E.Y -> 2
  else -> 3
}
`
	if got := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", guardedOnly); len(got) != 0 {
		t.Errorf("else is needed for the guarded entry, want no findings, got %d", len(got))
	}
	coveredAnyway := whenGuardDecls + `
fun a(e: E, flag: Boolean) = when (e) {
  E.X if flag -> 1
  E.X -> 4
  E.Y -> 2
  else -> 3
}
`
	if got := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", coveredAnyway); len(got) != 1 {
		t.Errorf("an unguarded E.X makes else redundant, want 1 finding, got %d", len(got))
	}
}
