package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
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

func TestDoubleMutabilityForCollection_NegativeKDocMutableFactoryMention(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
fun foo() {
    /**
     * mutableListOf<String>() is mentioned here.
     */
    var list = emptyList<String>()
}`)
	for _, f := range findings {
		if f.Rule == "DoubleMutabilityForCollection" {
			t.Error("should not flag mutable factory text in KDoc")
		}
	}
}

func TestDoubleMutabilityForCollection_NegativeStringMutableFactoryMention(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
fun foo() {
    var list = "mutableListOf<String>()"
}`)
	for _, f := range findings {
		if f.Rule == "DoubleMutabilityForCollection" {
			t.Error("should not flag mutable factory text in string initializer")
		}
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

func TestAvoidReferentialEquality_NegativeNullCheck(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test
fun bar(a: String?) {
    val missing = a === null
}
`)
	for _, f := range findings {
		if f.Rule == "AvoidReferentialEquality" {
			t.Error("should not flag referential null checks")
		}
	}
}

func TestAvoidReferentialEquality_NegativeCommentAndStringMentions(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test
fun bar(a: String, b: String) {
    // a === b
    val text = "a !== b"
    val same = a == b
}
`)
	for _, f := range findings {
		if f.Rule == "AvoidReferentialEquality" {
			t.Error("should not flag referential operators in comments or strings")
		}
	}
}

func TestAvoidReferentialEquality_NegativeSentinelObject(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

object DISPOSED

fun check(d: Any): Boolean {
    return d === DISPOSED
}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for sentinel object identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeSentinelAlias(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

object DISPOSED

fun dispose(current: Any): Boolean {
    val d: Any = DISPOSED
    return current !== d
}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for local alias to sentinel identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeViewIdentity(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "AvoidReferentialEquality", `
package test

class View
class Binding(val blockMultiSelectList: View)

fun check(view: View, binding: Binding): Boolean {
    return view !== binding.blockMultiSelectList
}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for view identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeSourceOnlyBindingViewIdentity(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Binding(val blockMultiSelectList: Any)

fun check(view: Any, binding: Binding): Boolean {
    return view !== binding.blockMultiSelectList
}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for source-only binding view identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeSourceOnlyHolderViewIdentity(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Holder(val itemView: Any)

fun check(candidate: Any, holder: Holder): Boolean {
    return candidate !== holder.itemView
}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for source-only holder view identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_PositivePreviewNameLookalike(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

fun check(preview: String, other: String): Boolean {
    return preview === other
}`)
	if len(findings) == 0 {
		t.Fatal("expected lowercase preview lookalike to still be reported")
	}
}

func TestAvoidReferentialEquality_NegativeThisFieldDelegateIdentity(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Tracker(private val bufferingTracker: Any) {
    private var delegate: Any = bufferingTracker

    fun isBuffered(candidate: Any): Boolean {
        return this.delegate === candidate
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for explicit this field identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeIterationIdentityIt(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

fun isLast(blocks: List<Any>, lastRichTextBlock: Any): Boolean {
    return blocks.any { it === lastRichTextBlock }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for iteration variable identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeIterationIdentityNamedParam(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Ref(val contributor: Any)

fun isActive(contributors: List<Any>, activeRef: Ref): Boolean {
    return contributors.any { contributor -> contributor === activeRef.contributor }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for named iteration parameter identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeArrayElementIdentity(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

fun currentBufferIndex(currentBuffer: Any, buffers: Array<Any>): Int {
    return if (currentBuffer === buffers[0]) 0 else 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for array element identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeResourceCleanupGuard(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Bitmap {
    fun recycle() = Unit
}

fun recycleOriginalIfCorrected(correctedBitmap: Bitmap, bitmap: Bitmap) {
    if (correctedBitmap !== bitmap) {
        bitmap.recycle()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for resource cleanup identity guard, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeSingletonTypeIdentity(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

object MKAppearance {
    fun merge(other: Any): Any = when {
        this === MKAppearance -> other
        other === MKAppearance -> this
        else -> other
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for singleton type identity checks, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeQualifiedSingletonIdentity(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

sealed class WorkspaceOverlayState {
    object Hidden : WorkspaceOverlayState()
    object Expanded : WorkspaceOverlayState()
}

fun isHidden(state: WorkspaceOverlayState): Boolean {
    return state === WorkspaceOverlayState.Hidden
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for qualified singleton object identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_PositiveQualifiedLowercasePropertyLookalike(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Holder(val hidden: String)

fun isHidden(state: String, holder: Holder): Boolean {
    return state === holder.hidden
}
`)
	if len(findings) == 0 {
		t.Fatal("expected lowercase qualified property comparison to still be reported")
	}
}

func TestAvoidReferentialEquality_PositiveLocalSentinelAliasLookalike(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

fun same(current: String, other: String): Boolean {
    val d: String = other
    return current !== d
}
`)
	if len(findings) == 0 {
		t.Fatal("expected non-sentinel local alias comparison to still be reported")
	}
}

func TestAvoidReferentialEquality_NegativeCompareToIdentityFastPath(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Row : Comparable<Row> {
    override fun compareTo(other: Row): Int {
        if (this === other) return 0
        return 1
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for compareTo identity fast path, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_PositiveNonLeadingCompareToIdentityCheck(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Row : Comparable<Row> {
    override fun compareTo(other: Row): Int {
        println("comparing")
        if (this === other) return 0
        return 1
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when referential equality is not the leading compareTo fast path")
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

func TestAvoidReferentialEquality_HonorsForbiddenTypePatterns(t *testing.T) {
	// ForbiddenTypePatterns was previously a dead config — exposed in
	// metadata but never consulted by the check, which used a more
	// permissive IsKnownValueType heuristic. With the wiring, the
	// resolver path gates firing on whether either operand's resolved
	// FQN matches one of the configured glob patterns. Default
	// `kotlin.String` is the documented default.
	var rule *rules.AvoidReferentialEqualityRule
	for _, candidate := range api.Registry {
		if candidate.ID == "AvoidReferentialEquality" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.AvoidReferentialEqualityRule)
			if !ok {
				t.Fatalf("expected AvoidReferentialEqualityRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("AvoidReferentialEquality rule not registered")
	}
	original := rule.ForbiddenTypePatterns
	defer func() { rule.ForbiddenTypePatterns = original }()

	// Default ForbiddenTypePatterns=["kotlin.String"]: with the
	// resolver active, a String === comparison must fire.
	stringCmp := `package test
fun bar(a: String, b: String): Boolean {
    return a === b
}
`
	if findings := runRuleByNameWithResolver(t, "AvoidReferentialEquality", stringCmp); len(findings) == 0 {
		t.Fatal("expected finding for String === String under default ForbiddenTypePatterns")
	}

	// User configures a pattern that matches a custom type by its
	// resolved name. krit's resolver returns the simple name for
	// user-defined types declared in the same file, so the pattern
	// is just `Foo`.
	rule.ForbiddenTypePatterns = []string{"Foo"}
	customCmp := `package test
class Foo
fun bar(a: Foo, b: Foo): Boolean {
    return a === b
}
`
	if findings := runRuleByNameWithResolver(t, "AvoidReferentialEquality", customCmp); len(findings) == 0 {
		t.Fatal("expected finding for Foo === Foo when pattern matches")
	}

	// Same comparison but with a non-matching pattern: rule does not
	// fire — the resolver returned a name and it doesn't match.
	rule.ForbiddenTypePatterns = []string{"kotlin.String"}
	if findings := runRuleByNameWithResolver(t, "AvoidReferentialEquality", customCmp); len(findings) != 0 {
		t.Fatalf("expected no findings when Foo's name doesn't match the configured patterns, got %d", len(findings))
	}

	// Glob wildcard `?oo` matches three-letter names ending in "oo".
	rule.ForbiddenTypePatterns = []string{"?oo"}
	if findings := runRuleByNameWithResolver(t, "AvoidReferentialEquality", customCmp); len(findings) == 0 {
		t.Fatal("expected finding under wildcard pattern '?oo'")
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

func TestCharArrayToStringCall_PositiveDirectFactoryReceiver(t *testing.T) {
	findings := runRuleByName(t, "CharArrayToStringCall", `
package test
fun bar() {
    val s = charArrayOf('a', 'b').toString()
}`)
	if len(findings) == 0 {
		t.Error("CharArrayToStringCall should flag toString() on direct charArrayOf receiver")
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

func TestCharArrayToStringCall_NegativeInitializerMentions(t *testing.T) {
	findings := runRuleByName(t, "CharArrayToStringCall", `
package test
fun bar() {
    // val chars = charArrayOf('a')
    val text = "charArrayOf('a')"
    val rendered = text.toString()
}`)
	for _, f := range findings {
		if f.Rule == "CharArrayToStringCall" {
			t.Error("should not flag when charArrayOf appears only in comments or strings")
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

// --- NoElseInWhenSealed ---

func TestNoElseInWhenSealed_MissingSealedVariant_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NoElseInWhenSealed", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val value: String) : Result()
    data class Failure(val error: Throwable) : Result()
}

fun render(r: Result): String = when (r) {
    is Result.Loading -> "loading"
    is Result.Success -> r.value
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for missing sealed variant")
	}
}

func TestNoElseInWhenSealed_AllVariantsCovered_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NoElseInWhenSealed", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val value: String) : Result()
    data class Failure(val error: Throwable) : Result()
}

fun render(r: Result): String = when (r) {
    is Result.Loading -> "loading"
    is Result.Success -> r.value
    is Result.Failure -> r.error.message ?: "error"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all sealed variants covered, got %d", len(findings))
	}
}

func TestNoElseInWhenSealed_HasElse_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NoElseInWhenSealed", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val value: String) : Result()
}

fun render(r: Result): String = when (r) {
    is Result.Loading -> "loading"
    else -> "ready"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when else branch present, got %d", len(findings))
	}
}

func TestNoElseInWhenSealed_MissingEnumEntry_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NoElseInWhenSealed", `
package test

enum class Color { RED, GREEN, BLUE }

fun describe(c: Color): String = when (c) {
    Color.RED -> "warm"
    Color.BLUE -> "cool"
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for missing enum entry")
	}
}

func TestNoElseInWhenSealed_AllEnumEntriesCovered_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NoElseInWhenSealed", `
package test

enum class Color { RED, GREEN, BLUE }

fun describe(c: Color): String = when (c) {
    Color.RED -> "warm"
    Color.GREEN -> "fresh"
    Color.BLUE -> "cool"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all enum entries covered, got %d", len(findings))
	}
}

func TestNoElseInWhenSealed_NotSealedOrEnum_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NoElseInWhenSealed", `
package test

fun classify(x: Int): String = when (x) {
    1 -> "one"
    2 -> "two"
    else -> "many"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on non-sealed/non-enum subject, got %d", len(findings))
	}
}

func TestNoElseInWhenSealed_NoSubject_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NoElseInWhenSealed", `
package test

fun classify(x: Int): String = when {
    x > 0 -> "positive"
    x < 0 -> "negative"
    else -> "zero"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on subject-less when, got %d", len(findings))
	}
}

// --- NonExhaustiveWhen ---

func TestNonExhaustiveWhen_ExpressionBody_MissingSealed_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val v: String) : Result()
    data class Failure(val e: Throwable) : Result()
}

fun render(r: Result): String = when (r) {
    is Result.Loading -> "loading"
    is Result.Success -> r.v
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for non-exhaustive when used as expression body")
	}
}

func TestNonExhaustiveWhen_StatementForm_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val v: String) : Result()
    data class Failure(val e: Throwable) : Result()
}

fun handle(r: Result) {
    when (r) {
        is Result.Loading -> println("loading")
        is Result.Success -> println(r.v)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for statement-form when, got %d", len(findings))
	}
}

func TestNonExhaustiveWhen_PropertyInit_MissingEnum_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

enum class Color { RED, GREEN, BLUE }

fun describe(c: Color): String {
    val label: String = when (c) {
        Color.RED -> "warm"
        Color.BLUE -> "cool"
    }
    return label
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for non-exhaustive when in property initializer")
	}
}

func TestNonExhaustiveWhen_Return_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val v: String) : Result()
    data class Failure(val e: Throwable) : Result()
}

fun pick(r: Result): Int {
    return when (r) {
        is Result.Loading -> 1
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for non-exhaustive when in return")
	}
}

func TestNonExhaustiveWhen_BooleanMissingFalse_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

fun toInt(b: Boolean): Int = when (b) {
    true -> 1
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for Boolean when missing false branch")
	}
}

func TestNonExhaustiveWhen_BooleanFullyCovered_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

fun toInt(b: Boolean): Int = when (b) {
    true -> 1
    false -> 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when both Boolean branches present, got %d", len(findings))
	}
}

func TestNonExhaustiveWhen_HasElse_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val v: String) : Result()
    data class Failure(val e: Throwable) : Result()
}

fun pick(r: Result): Int = when (r) {
    is Result.Loading -> 1
    else -> 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when else present, got %d", len(findings))
	}
}

func TestNonExhaustiveWhen_AllVariantsCovered_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val v: String) : Result()
    data class Failure(val e: Throwable) : Result()
}

fun pick(r: Result): Int = when (r) {
    is Result.Loading -> 1
    is Result.Success -> 2
    is Result.Failure -> 3
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all sealed variants covered, got %d", len(findings))
	}
}

func TestNonExhaustiveWhen_CallArgument_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NonExhaustiveWhen", `
package test

sealed class Result {
    object Loading : Result()
    data class Success(val v: String) : Result()
    data class Failure(val e: Throwable) : Result()
}

fun log(r: Result) {
    println(when (r) {
        is Result.Loading -> "a"
    })
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for when used as call argument")
	}
}
