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

func TestEqualsWithHashCodeExist_SkipsClassesWithGeneratedMembers(t *testing.T) {
	for _, header := range []string{"data class Foo(val id: Int)", "@JvmInline value class Foo(val id: Int)"} {
		for _, member := range []string{
			"override fun hashCode(): Int = 1",
			"override fun equals(other: Any?): Boolean = true",
		} {
			findings := runRuleByName(t, "EqualsWithHashCodeExist", "package test\n"+header+" {\n    "+member+"\n}\n")
			if len(findings) != 0 {
				t.Errorf("%s with %q: expected no findings, got %d", header, member, len(findings))
			}
		}
	}
}

func TestEqualsWithHashCodeExist_StillReportsPlainClassOverridingOnlyHashCode(t *testing.T) {
	findings := runRuleByName(t, "EqualsWithHashCodeExist", `
package test
class Foo(val id: Int) {
    override fun hashCode(): Int = id
}`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

// --- DontDowncastCollectionTypes ---

func TestDontDowncastCollectionTypes_FollowsDetekt(t *testing.T) {
	flagged := map[string]string{
		"as MutableList":    "fun f(l: List<String>) { val m = l as MutableList<String> }",
		"is MutableList":    "fun f(l: List<String>) { val m = l is MutableList<*> }",
		"not is ArrayList":  "fun f(l: List<String>) { val m = l !is ArrayList<String> }",
		"safe cast":         "fun f(l: Set<String>) { val m = l as? HashSet<String> }",
		"nullable source":   "fun f(l: List<String>?) { val m = l as MutableList<String> }",
		"map to hash map":   "fun f(l: Map<String, Int>) { val m = l as HashMap<String, Int> }",
		"qualified array":   "fun f(l: List<String>) { val m = l as java.util.ArrayList<String> }",
		"parenthesized lhs": "fun f(l: List<String>) { val m = (l) as MutableList<String> }",
	}
	for name, src := range flagged {
		if got := runRuleByNameWithResolver(t, "DontDowncastCollectionTypes", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"collection source": "fun f(l: Collection<String>) { val m = l as ArrayList<String> }",
		"iterable source":   "fun f(l: Iterable<String>) { val m = l as MutableList<String> }",
		"mutable source":    "fun f(l: MutableList<String>?) { val m = l as MutableList<String> }",
		"nullable target":   "fun f(l: Set<String>) { val m = l as MutableSet<String>? }",
		"any source":        "fun f(l: Any) { val m = l as MutableList<String> }",
		"copying call":      "fun f(l: List<String>) { val m = l.toMutableList() }",
	}
	for name, src := range clean {
		if got := runRuleByNameWithResolver(t, "DontDowncastCollectionTypes", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
	got := runRuleByNameWithResolver(t, "DontDowncastCollectionTypes", "package test\nfun f(l: List<String>) { val m = l as MutableList<String> }\n")
	if len(got) == 1 && got[0].Message != "Down-casting from type List to MutableList is risky. Use `toMutableList()` instead." {
		t.Errorf("unexpected message %q", got[0].Message)
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
	findings := runRuleByNameWithResolver(t, "ImplicitUnitReturnType", `
package test
fun clearAll(l: MutableList<Int>) = l.clear()
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for an expression-body function that returns Unit, got %d", len(findings))
	}
	if findings[0].Fix == nil || findings[0].Fix.Replacement != ": Unit" {
		t.Errorf("expected a fix inserting ': Unit', got %+v", findings[0].Fix)
	}
}

func TestImplicitUnitReturnType_BranchingBodies(t *testing.T) {
	cases := map[string]string{
		"when with Unit branches": "fun pick(l: MutableList<Int>, b: Int) = when (b) { 1 -> l.clear(); else -> {} }",
		"if with Unit branches":   "fun pick(l: MutableList<Int>, b: Boolean) = if (b) l.clear() else { Unit }",
	}
	for name, src := range cases {
		if got := runRuleByNameWithResolver(t, "ImplicitUnitReturnType", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
}

func TestImplicitUnitReturnType_Negative(t *testing.T) {
	cases := map[string]string{
		"explicit Unit":       "fun clearAll(l: MutableList<Int>): Unit = l.clear()",
		"block body":          "fun clearAll(l: MutableList<Int>) {\n    l.clear()\n}",
		"value expression":    "fun first(l: List<Int>) = l.size",
		"explicit other type": "fun clearAll(l: MutableList<Int>): Any = l.clear()",
		"Unit constant":       "fun nothing() = Unit",
		"branch with a value": "fun pick(l: MutableList<Int>, b: Boolean) = if (b) l.clear() else 1",
		"when without else":   "fun pick(l: MutableList<Int>, b: Int) = when (b) { 1 -> l.clear() }",
	}
	for name, src := range cases {
		if got := runRuleByNameWithResolver(t, "ImplicitUnitReturnType", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
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

func TestAvoidReferentialEquality_NegativeSameInstanceShortCircuit(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test
class Node(val text: String) {
    fun isSelf(candidate: Node) = candidate === this
}
fun update(text: String, original: String): Boolean {
    if (normalize(text) === text) return false
    return text === original
}
`)
	for _, f := range findings {
		if f.Rule == "AvoidReferentialEquality" {
			t.Errorf("should not flag same-instance short circuits: %v", f)
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
class Binding(val totalsPanel: View)

fun check(view: View, binding: Binding): Boolean {
    return view !== binding.totalsPanel
}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for view identity check, got %d", len(findings))
	}
}

func TestAvoidReferentialEquality_NegativeSourceOnlyBindingViewIdentity(t *testing.T) {
	findings := runRuleByName(t, "AvoidReferentialEquality", `
package test

class Binding(val totalsPanel: Any)

fun check(view: Any, binding: Binding): Boolean {
    return view !== binding.totalsPanel
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

class Ledger(private val primaryEntry: Any) {
    private var current: Any = primaryEntry

    fun isCurrent(candidate: Any): Boolean {
        return this.current === candidate
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

fun isFinal(steps: List<Any>, finalStep: Any): Boolean {
    return steps.any { it === finalStep }
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

class Scan {
    fun recycle() = Unit
}

fun releaseIfRotated(rotatedScan: Scan, scan: Scan) {
    if (rotatedScan !== scan) {
        scan.recycle()
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

object DefaultTheme {
    fun merge(other: Any): Any = when {
        this === DefaultTheme -> other
        other === DefaultTheme -> this
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

sealed class DrawerState {
    object Closed : DrawerState()
    object Open : DrawerState()
}

fun isClosed(state: DrawerState): Boolean {
    return state === DrawerState.Closed
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

class Row(val label: String) : Comparable<Row> {
    override fun compareTo(other: Row): Int {
        println("comparing")
        if (label === other.label) return 0
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

func TestWrongEqualsTypeParameter_Overload(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"overload-string", "class Foo {\n    fun equals(other: Foo): Boolean = true\n}", 1},
		{"overload-non-null-any", "class Foo {\n    fun equals(other: Any): Boolean = true\n}", 1},
		{"extension", "fun Foo.equals(other: Foo): Boolean = true\nclass Foo", 0},
		{"two-params", "class Foo {\n    fun equals(a: Foo, b: Foo): Boolean = true\n}", 0},
		{"no-params", "class Foo {\n    fun equals(): Boolean = true\n}", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "WrongEqualsTypeParameter", "package test\n"+tc.src)
			if len(findings) != tc.want {
				t.Fatalf("want %d findings, got %d", tc.want, len(findings))
			}
		})
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

func TestDoubleMutabilityForCollection_SkipsNullableLazyInit(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
class Cache {
    private var items: MutableList<String>? = null
    fun load() {
        var local: MutableList<String>? = null
        local = mutableListOf()
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for nullable lazy-init vars, got %v", findings)
	}
}

func TestDoubleMutabilityForCollection_SkipsReassignedVars(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
class Cache {
    private var items: MutableList<String> = mutableListOf()
    fun reset() {
        this.items = mutableListOf()
    }
    fun run() {
        var local: MutableList<String> = mutableListOf()
        local = mutableListOf()
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for reassigned vars, got %v", findings)
	}
}

func TestDoubleMutabilityForCollection_SkipsNonPrivateMembers(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
class Cache {
    var open: MutableList<String> = mutableListOf()
    internal var shared: MutableList<String> = mutableListOf()
    protected var inherited: MutableList<String> = mutableListOf()
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-private vars, got %v", findings)
	}
}

func TestDoubleMutabilityForCollection_PositivePrivateNeverReassigned(t *testing.T) {
	findings := runRuleByName(t, "DoubleMutabilityForCollection", `
package test
class Cache {
    private var items: MutableList<String> = mutableListOf()
}
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %v", findings)
	}
}

func TestEqualsWithHashCodeExist_Objects(t *testing.T) {
	src := "package test\nobject A { override fun equals(other: Any?): Boolean = other is A }\n" +
		"class B {\n companion object {\n override fun equals(other: Any?): Boolean = other is B\n }\n}\n" +
		"object C { override fun equals(other: Any?): Boolean = other is C\n override fun hashCode(): Int = 1 }\n" +
		"data object D { override fun equals(other: Any?): Boolean = other is D }\n"
	if got := runRuleByName(t, "EqualsWithHashCodeExist", src); len(got) != 2 {
		t.Fatalf("expected findings for object A and the companion only, got %d: %v", len(got), got)
	}
}

func TestEqualsRules_RequireTheAnyNullableSignature(t *testing.T) {
	// A hashCode override without a matching equals(Any?) is incomplete, and a differently typed equals is not
	// the equals() contract detekt checks.
	other := "package test\nclass A { override fun equals(other: String?): Boolean = true\n override fun hashCode() = 1 }\n"
	if got := runRuleByName(t, "EqualsWithHashCodeExist", other); len(got) != 1 {
		t.Errorf("EqualsWithHashCodeExist with equals(String?): want 1 finding, got %d", len(got))
	}
	if got := runRuleByName(t, "EqualsAlwaysReturnsTrueOrFalse", other); len(got) != 0 {
		t.Errorf("EqualsAlwaysReturnsTrueOrFalse with equals(String?): want no findings, got %d", len(got))
	}
	receiver := "package test\nclass A { fun String.equals(o: Any?): Boolean = true\n override fun hashCode() = 1 }\n"
	if got := runRuleByName(t, "EqualsAlwaysReturnsTrueOrFalse", receiver); len(got) != 0 {
		t.Errorf("an extension equals is not the contract: want no findings, got %d", len(got))
	}
}
