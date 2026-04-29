package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// --- EqualsAlwaysReturnsTrueOrFalse ---

func TestEqualsAlwaysReturnsTrueOrFalse_Positive(t *testing.T) {
	findings := runRuleByName(t, "EqualsAlwaysReturnsTrueOrFalse", `
package test
class Foo {
    override fun equals(other: Any?): Boolean = true
}`)
	if len(findings) == 0 {
		t.Error("expected finding for equals() that always returns true")
	}
}

func TestEqualsAlwaysReturnsTrueOrFalse_Negative(t *testing.T) {
	findings := runRuleByName(t, "EqualsAlwaysReturnsTrueOrFalse", `
package test
class Foo {
    override fun equals(other: Any?): Boolean {
        return this.id == (other as? Foo)?.id
    }
}`)
	for _, f := range findings {
		if f.Rule == "EqualsAlwaysReturnsTrueOrFalse" {
			t.Error("should not flag equals() with real logic")
		}
	}
}

// --- EqualsWithHashCodeExist ---

func TestEqualsWithHashCodeExist_Positive(t *testing.T) {
	findings := runRuleByName(t, "EqualsWithHashCodeExist", `
package test
class Foo {
    override fun equals(other: Any?): Boolean {
        return this === other
    }
}`)
	if len(findings) == 0 {
		t.Error("expected finding for equals without hashCode")
	}
}

func TestEqualsWithHashCodeExist_Negative(t *testing.T) {
	findings := runRuleByName(t, "EqualsWithHashCodeExist", `
package test
class Foo {
    override fun equals(other: Any?): Boolean {
        return this === other
    }
    override fun hashCode(): Int {
        return id.hashCode()
    }
}`)
	for _, f := range findings {
		if f.Rule == "EqualsWithHashCodeExist" {
			t.Error("should not flag when both equals and hashCode are present")
		}
	}
}

// --- UnreachableCode ---

func TestUnreachableCode_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test
fun foo(): Int {
    return 1
    println("unreachable")
}`)
	if len(findings) == 0 {
		t.Error("expected finding for code after return")
	}
}

func TestUnreachableCode_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test
fun foo(): Int {
    println("reachable")
    return 1
}`)
	for _, f := range findings {
		if f.Rule == "UnreachableCode" {
			t.Error("should not flag reachable code")
		}
	}
}

// --- DontDowncastCollectionTypes ---

func TestDontDowncastCollectionTypes_Positive(t *testing.T) {
	findings := runRuleByName(t, "DontDowncastCollectionTypes", `
package test
fun foo(list: List<String>) {
    val mutable = list as MutableList<String>
}`)
	if len(findings) == 0 {
		t.Error("expected finding for downcasting List to MutableList")
	}
}

func TestDontDowncastCollectionTypes_Negative(t *testing.T) {
	findings := runRuleByName(t, "DontDowncastCollectionTypes", `
package test
fun foo(list: List<String>) {
    val mutable = list.toMutableList()
}`)
	for _, f := range findings {
		if f.Rule == "DontDowncastCollectionTypes" {
			t.Error("should not flag toMutableList() call")
		}
	}
}

// --- DoubleMutabilityForCollection ---

func TestDoubleMutabilityForCollection_Positive(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
fun foo() {
    var list: MutableList<String> = mutableListOf()
}`)
	if len(findings) == 0 {
		t.Error("expected finding for var with MutableList (double mutability)")
	}
}

func TestDoubleMutabilityForCollection_Negative(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
fun foo() {
    val list: MutableList<String> = mutableListOf()
}`)
	for _, f := range findings {
		if f.Rule == "DoubleMutabilityForCollection" {
			t.Error("should not flag val with MutableList")
		}
	}
}

func TestDoubleMutabilityForCollection_NegativeWrapperTypeName(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
class MyMutableListWrapper
fun foo() {
    var list: MyMutableListWrapper = MyMutableListWrapper()
}`)
	for _, f := range findings {
		if f.Rule == "DoubleMutabilityForCollection" {
			t.Error("should not flag wrapper types whose names merely contain mutable collection substrings")
		}
	}
}

func TestDoubleMutabilityForCollection_NegativeLocalMutableListLookalike(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
class MutableList<T>
fun foo() {
    var list: MutableList<String> = MutableList()
}`)
	for _, f := range findings {
		if f.Rule == "DoubleMutabilityForCollection" {
			t.Error("should not flag a same-file MutableList lookalike")
		}
	}
}

func TestDoubleMutabilityForCollection_NegativeImportedMutableListLookalike(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
import sample.MutableList
fun foo() {
    var list: MutableList<String> = TODO()
}`)
	for _, f := range findings {
		if f.Rule == "DoubleMutabilityForCollection" {
			t.Error("should not flag an imported MutableList lookalike")
		}
	}
}

func TestDoubleMutabilityForCollection_PositiveFactoryWithoutExplicitType(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
fun foo() {
    var list = mutableListOf<String>()
}`)
	if len(findings) == 0 {
		t.Error("expected finding for var initialized with mutableListOf()")
	}
}

// --- ImplicitUnitReturnType ---

func TestImplicitUnitReturnType_Positive(t *testing.T) {
	findings := runRuleByName(t, "ImplicitUnitReturnType", `
package test
fun doSomething() {
    println("hello")
}`)
	if len(findings) == 0 {
		t.Error("expected finding for function without explicit return type")
	}
}

func TestImplicitUnitReturnType_Negative(t *testing.T) {
	findings := runRuleByName(t, "ImplicitUnitReturnType", `
package test
fun doSomething(): Unit {
    println("hello")
}`)
	for _, f := range findings {
		if f.Rule == "ImplicitUnitReturnType" {
			t.Error("should not flag function with explicit Unit return type")
		}
	}
}

func TestImplicitUnitReturnType_IgnoresTestFunctions(t *testing.T) {
	findings := runRuleByNameOnPath(t, "ImplicitUnitReturnType", "src/test/kotlin/FooTest.kt", `
package test

class FooTest {
    @Test
    fun doesSomethingUseful() {
        println("hello")
    }
}
`)
	for _, f := range findings {
		if f.Rule == "ImplicitUnitReturnType" {
			t.Errorf("ImplicitUnitReturnType should ignore test functions, got: %s", f.Message)
		}
	}
}

func TestImplicitUnitReturnType_IgnoresOverrides(t *testing.T) {
	findings := runRuleByName(t, "ImplicitUnitReturnType", `
package test
class Foo : Runnable {
    override fun run() {
        println("hello")
    }
}
`)
	for _, f := range findings {
		if f.Rule == "ImplicitUnitReturnType" {
			t.Errorf("ImplicitUnitReturnType should ignore overrides, got: %s", f.Message)
		}
	}
}

// --- AvoidReferentialEquality ---

func TestAvoidReferentialEquality_Positive(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test
fun bar() {
    val a = "hello"
    val b = "hello"
    val same = a === b
}`)
	if len(findings) == 0 {
		t.Error("AvoidReferentialEquality should flag === usage")
	}
}

func TestAvoidReferentialEquality_NotEquals(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test
fun bar() {
    val a = "hello"
    val b = "world"
    val diff = a !== b
}`)
	if len(findings) == 0 {
		t.Error("AvoidReferentialEquality should flag !== usage")
	}
}

func TestAvoidReferentialEquality_Negative(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test
fun bar() {
    val a = "hello"
    val b = "hello"
    val same = a == b
}`)
	for _, f := range findings {
		if f.Rule == "AvoidReferentialEquality" {
			t.Error("should not flag structural equality ==")
		}
	}
}

// Verifies the fix is pinned to the operator child, not a strings.Replace
// on the whole node text. With the old implementation, an equality
// expression whose operand contained "!==" inside a string literal would
// get the string corrupted instead of the operator replaced.
func TestAvoidReferentialEquality_FixPinsToOperatorOnly(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test
fun bar(): Boolean {
    val a = "hello!==world"
    val b = "hello!==world"
    return a === b
}`)
	var ref *scanner.Finding
	for i := range findings {
		if findings[i].Rule == "AvoidReferentialEquality" {
			ref = &findings[i]
			break
		}
	}
	if ref == nil {
		t.Fatal("expected AvoidReferentialEquality finding")
	}
	if ref.Fix == nil {
		t.Fatal("expected fix to be present")
	}
	if !ref.Fix.ByteMode {
		t.Error("expected byte-mode fix")
	}
	if ref.Fix.Replacement != "==" {
		t.Errorf("expected replacement to be just the operator %q, got %q", "==", ref.Fix.Replacement)
	}
	// The byte range must span exactly 3 bytes (the === operator),
	// not the full equality_expression.
	if span := ref.Fix.EndByte - ref.Fix.StartByte; span != 3 {
		t.Errorf("expected replacement byte range to cover the 3-byte === operator, got span %d", span)
	}
}

// --- CharArrayToStringCall ---

func TestCharArrayToStringCall_Positive(t *testing.T) {
	findings := runRuleByName(t, "CharArrayToStringCall", `
package test
fun bar() {
    val charArray = charArrayOf('a', 'b', 'c')
    val s = charArray.toString()
}`)
	if len(findings) == 0 {
		t.Error("CharArrayToStringCall should flag charArray.toString()")
	}
}

func TestCharArrayToStringCall_Negative(t *testing.T) {
	findings := runRuleByName(t, "CharArrayToStringCall", `
package test
fun bar() {
    val charArray = charArrayOf('a', 'b', 'c')
    val s = String(charArray)
}`)
	for _, f := range findings {
		if f.Rule == "CharArrayToStringCall" {
			t.Error("should not flag String(charArray)")
		}
	}
}

func TestCharArrayToStringCall_PositiveExplicitType(t *testing.T) {
	findings := runRuleByName(t, "CharArrayToStringCall", `
package test
fun bar() {
    val arr: CharArray = charArrayOf('a', 'b')
    val s = arr.toString()
}`)
	if len(findings) == 0 {
		t.Error("CharArrayToStringCall should flag toString() on explicitly-typed CharArray variable")
	}
}

func TestCharArrayToStringCall_NegativeStringReceiver(t *testing.T) {
	findings := runRuleByName(t, "CharArrayToStringCall", `
package test
fun bar() {
    val s: String = "hello"
    val t = s.toString()
}`)
	for _, f := range findings {
		if f.Rule == "CharArrayToStringCall" {
			t.Error("should not flag toString() on a String receiver")
		}
	}
}

// --- WrongEqualsTypeParameter ---

func TestWrongEqualsTypeParameter_Positive(t *testing.T) {
	findings := runRuleByName(t, "WrongEqualsTypeParameter", `
package test
class Foo {
    override fun equals(other: String): Boolean {
        return false
    }
}`)
	if len(findings) == 0 {
		t.Error("WrongEqualsTypeParameter should flag equals(other: String)")
	}
}

func TestWrongEqualsTypeParameter_Negative(t *testing.T) {
	findings := runRuleByName(t, "WrongEqualsTypeParameter", `
package test
class Foo {
    override fun equals(other: Any?): Boolean {
        return this === other
    }
}`)
	for _, f := range findings {
		if f.Rule == "WrongEqualsTypeParameter" {
			t.Error("should not flag equals(other: Any?)")
		}
	}
}

func TestWrongEqualsTypeParameter_NegativeNotEquals(t *testing.T) {
	findings := runRuleByName(t, "WrongEqualsTypeParameter", `
package test
class Foo {
    fun notEquals(other: String): Boolean {
        return false
    }
}`)
	for _, f := range findings {
		if f.Rule == "WrongEqualsTypeParameter" {
			t.Error("should not flag a non-equals function")
		}
	}
}

func TestWrongEqualsTypeParameter_NegativeNoOverride(t *testing.T) {
	findings := runRuleByName(t, "WrongEqualsTypeParameter", `
package test
class Foo {
    fun equals(other: String): Boolean {
        return false
    }
}`)
	for _, f := range findings {
		if f.Rule == "WrongEqualsTypeParameter" {
			t.Error("should not flag equals without override modifier")
		}
	}
}
