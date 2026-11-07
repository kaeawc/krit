package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// --- ExpressionBodySyntax ---

func TestExpressionBodySyntax_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExpressionBodySyntax", `
package test
fun double(x: Int): Int {
    return x * 2
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for single-return body convertible to expression syntax")
	}
}

func TestExpressionBodySyntax_ReportsReturnAndAccessors(t *testing.T) {
	findings := runRuleByName(t, "ExpressionBodySyntax", `package test
class Box {
    val size: Int
        get() {
            return 1
        }

    fun double(x: Int): Int {
        return x * 2
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected a finding for the getter and the function, got %d", len(findings))
	}
	if l0, l1 := findings[0].Line, findings[1].Line; (l0 != 5 || l1 != 9) && (l0 != 9 || l1 != 5) {
		t.Errorf("findings must sit on the return statements, got lines %d and %d", l0, l1)
	}
	if !strings.Contains(findings[0].Message, "exact one statement") {
		t.Errorf("unexpected message %q", findings[0].Message)
	}
}

func TestExpressionBodySyntax_SkipsReturnNestedInElvis(t *testing.T) {
	findings := runRuleByName(t, "ExpressionBodySyntax", `package test
fun first(x: Int?): Int {
    return x ?: return 2
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when a return sits inside an elvis, got %d", len(findings))
	}
}

func TestExpressionBodySyntax_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExpressionBodySyntax", `
package test
fun double(x: Int): Int = x * 2
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for expression body, got %d", len(findings))
	}
}

func TestExpressionBodySyntax_HonorsIncludeLineWrapping(t *testing.T) {
	// IncludeLineWrapping was previously a dead config — exposed in
	// metadata but never consulted. Configure it via the rule pointer
	// and verify multi-line single-return bodies are flagged only when
	// the option is on.
	var rule *rules.ExpressionBodySyntaxRule
	for _, candidate := range api.Registry {
		if candidate.ID == "ExpressionBodySyntax" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.ExpressionBodySyntaxRule)
			if !ok {
				t.Fatalf("expected ExpressionBodySyntaxRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("ExpressionBodySyntax rule not registered")
	}
	original := rule.IncludeLineWrapping
	defer func() { rule.IncludeLineWrapping = original }()

	multiLineCode := `package test
fun render(name: String): String {
    return name
        .trim()
        .lowercase()
}
`
	// Default (false): multi-line return body NOT flagged.
	if findings := runRuleByName(t, "ExpressionBodySyntax", multiLineCode); len(findings) != 0 {
		t.Fatalf("expected no findings for multi-line body under IncludeLineWrapping=false, got %d", len(findings))
	}

	rule.IncludeLineWrapping = true

	// IncludeLineWrapping=true: same multi-line body is flagged.
	if findings := runRuleByName(t, "ExpressionBodySyntax", multiLineCode); len(findings) == 0 {
		t.Fatal("expected finding for multi-line body when IncludeLineWrapping=true")
	}

	// A non-return-only multi-line body still doesn't fire — IncludeLineWrapping
	// only affects the single-return-statement case.
	multiStatement := `package test
fun work() {
    println("a")
    println("b")
}
`
	if findings := runRuleByName(t, "ExpressionBodySyntax", multiStatement); len(findings) != 0 {
		t.Fatalf("expected no findings for multi-statement body even with IncludeLineWrapping=true, got %d", len(findings))
	}
}

func TestExpressionBodySyntax_IgnoresTestSources(t *testing.T) {
	findings := runRuleByNameOnPath(t, "ExpressionBodySyntax", "src/androidTest/kotlin/TestRunner.kt", `
package test
fun runner(): Runner {
    return Runner()
}
class Runner
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for test sources, got %d", len(findings))
	}
}

// --- ReturnCount ---

func TestReturnCount_Positive(t *testing.T) {
	findings := runRuleByName(t, "ReturnCount", `
package test
fun classify(x: Int, y: Int): String {
    println("starting classify")
    val result = if (x > y) {
        if (x > 100) {
            return "big"
        }
        println("x bigger")
        "x"
    } else {
        if (y < 0) {
            return "negative"
        }
        if (y == 0) {
            return "zero"
        }
        if (y > 1000) {
            return "huge"
        }
        println("y bigger")
        "y"
    }
    return result
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for function with many non-guard returns")
	}
}

func TestReturnCount_Negative(t *testing.T) {
	findings := runRuleByName(t, "ReturnCount", `
package test
fun isPositive(x: Int): Boolean {
    if (x > 0) return true
    return false
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for 2 returns (within limit), got %d", len(findings))
	}
}

func TestReturnCount_DefaultCountsGuardClauseReturns(t *testing.T) {
	findings := runRuleByName(t, "ReturnCount", `
package test
fun classify(x: Int): String {
    if (x < 0) return "negative"
    if (x == 0) return "zero"
    return "positive"
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for 3 returns including guard clauses")
	}
}

// --- ThrowsCount ---

func TestThrowsCount_Positive(t *testing.T) {
	findings := runRuleByName(t, "ThrowsCount", `
package test
fun validate(x: Int) {
    if (x < 0) throw IllegalArgumentException("negative")
    if (x == 0) throw IllegalStateException("zero")
    if (x > 100) throw RuntimeException("too big")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for function with 3 throw statements (max 2)")
	}
}

func TestThrowsCount_Negative(t *testing.T) {
	findings := runRuleByName(t, "ThrowsCount", `
package test
fun validate(x: Int) {
    if (x < 0) throw IllegalArgumentException("negative")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for 1 throw (within limit), got %d", len(findings))
	}
}

func TestThrowsCount_SkipsOverrideFunctions(t *testing.T) {
	body := `
    if (x < 0) throw IllegalArgumentException("negative")
    if (x == 0) throw IllegalStateException("zero")
    if (x > 100) throw RuntimeException("too big")
`
	if f := runRuleByName(t, "ThrowsCount", "package test\nclass A : B {\n override fun run(x: Int) {"+body+"}\n}\n"); len(f) != 0 {
		t.Fatalf("detekt skips override functions, got %d", len(f))
	}
	if f := runRuleByName(t, "ThrowsCount", "package test\nclass A : B {\n fun run(x: Int) {"+body+"}\n}\n"); len(f) != 1 {
		t.Fatalf("a non-override member should still report, got %d", len(f))
	}
}

// --- CollapsibleIfStatements ---

func TestCollapsibleIfStatements_Positive(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun check(a: Boolean, b: Boolean) {
    if (a) {
        if (b) {
            println("both")
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for collapsible nested ifs")
	}
}

func TestCollapsibleIfStatements_FixSimpleIdentifierCondition(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun check(a: Boolean, b: Boolean) {
    if (a) {
        if (b) {
            println("both")
        }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix to be populated for simple_identifier conditions (inner cond is a bare identifier, not parenthesized_expression)")
	}
	if !strings.Contains(findings[0].Fix.Replacement, "if (a && b)") {
		t.Errorf("expected merged condition 'if (a && b)', got %q", findings[0].Fix.Replacement)
	}
}

func TestCollapsibleIfStatements_FixComparisonCondition(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun check(x: Int, y: Int) {
    if (x > 0) {
        if (y > 0) {
            println("pos")
        }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix to be populated for comparison conditions")
	}
	if !strings.Contains(findings[0].Fix.Replacement, "if (x > 0 && y > 0)") {
		t.Errorf("expected merged condition 'if (x > 0 && y > 0)', got %q", findings[0].Fix.Replacement)
	}
}

func TestCollapsibleIfStatements_Negative(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun check(a: Boolean, b: Boolean) {
    if (a && b) {
        println("both")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for already-merged condition, got %d", len(findings))
	}
}

func TestCollapsibleIfStatements_InnerElseNegative(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun check(a: Boolean, b: Boolean) {
    if (a) {
        if (b) {
            println("both")
        } else {
            println("only a")
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when nested if has else, got %d", len(findings))
	}
}

func TestCollapsibleIfStatements_MultipleStatementsNegative(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun check(a: Boolean, b: Boolean) {
    if (a) {
        println("before")
        if (b) {
            println("both")
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when outer body has multiple statements, got %d", len(findings))
	}
}

// --- SafeCast ---

// runRedundantSafeCast runs SafeCast with the krit-only redundant cast check enabled.
func runRedundantSafeCast(t *testing.T, code string) []scanner.Finding {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID != "SafeCast" {
			continue
		}
		rule, ok := r.Implementation.(*rules.SafeCastRule)
		if !ok {
			t.Fatalf("SafeCast implementation is %T", r.Implementation)
		}
		rule.ReportRedundantCasts = true
		defer func() { rule.ReportRedundantCasts = false }()
		file := parseInline(t, code)
		cols := rules.NewDispatcher([]*api.Rule{r}, nil).Run(file)
		return cols.Findings()
	}
	t.Fatal("SafeCast not registered")
	return nil
}

func TestSafeCast_RedundantCastsAreOffByDefault(t *testing.T) {
	findings := runRuleByName(t, "SafeCast", `
package test
fun process(obj: Any) {
    if (obj is String) {
        val s = obj as String
        println(s)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not report a cast after an is-check, got %v", findings)
	}
}

func TestSafeCast_DetektDocumentedCases(t *testing.T) {
	findings := runRuleByName(t, "SafeCast", `
package test
fun noncompliant(element: Any): Int? {
    return if (element is Int) element else null
}
fun noncompliantNegated(element: Any): Int? {
    return if (element !is Int) null else element
}
fun compliant(element: Any): Int? = element as? Int
fun otherName(element: Any, other: Any): Int? = if (element is Int) other as Int else null
fun nullableTarget(element: Any): Int? = if (element is Int?) element else null
`)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %v", findings)
	}
	for _, f := range findings {
		if f.Message != "This cast should be replaced with a safe cast: as?" {
			t.Errorf("unexpected message %q", f.Message)
		}
	}
}

func TestSafeCast_Positive(t *testing.T) {
	findings := runRedundantSafeCast(t, `
package test
fun process(obj: Any) {
    if (obj is String) {
        val s = obj as String
        println(s)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for is-check followed by unsafe cast")
	}
}

func TestSafeCast_Negative(t *testing.T) {
	findings := runRuleByName(t, "SafeCast", `
package test
fun process(obj: Any) {
    val s = obj as? String
    println(s)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast, got %d", len(findings))
	}
}

func TestSafeCast_UnstableSubjectAndMixedElseIgnored(t *testing.T) {
	findings := runRedundantSafeCast(t, `
package test
class Holder {
    var mutable: Any? = null
    val lazyValue: Any by lazy { 1 }
    fun a() = if (mutable is String) mutable as String else null
    fun b() = if (lazyValue is String) lazyValue as String else null
}
fun c(x: Any) = if (x is String) x as String else x as Int
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
	stable := runRedundantSafeCast(t, `
package test
fun d(x: Any) = if (x is String) x as String else null
`)
	if len(stable) != 1 {
		t.Fatalf("expected the stable parameter to be flagged, got %d", len(stable))
	}
}

// --- VarCouldBeVal ---

func TestVarCouldBeVal_Positive(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
fun example() {
    var x = 42
    println(x)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for var that is never reassigned")
	}
}

func TestVarCouldBeVal_SkipsNonPrivateEnumMember(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
enum class Level {
    LOW,
    HIGH;

    var cachedId: String? = null
    var weight = -1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for public enum member vars, got %d", len(findings))
	}
}

func TestVarCouldBeVal_ReportsPrivateEnumMember(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
enum class Level {
    LOW,
    HIGH;

    private var weight = -1
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for private enum member var, got %d", len(findings))
	}
}

func TestVarCouldBeVal_Negative(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
fun example() {
    var x = 0
    x = 42
    println(x)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for var that is reassigned, got %d", len(findings))
	}
}

func TestVarCouldBeVal_DoesNotTreatOtherReceiverAssignmentAsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Box(var x: Int)
fun example(other: Box) {
    var x = 0
    other.x = 42
    println(x)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for local var when only another receiver's property is reassigned")
	}
}

func TestVarCouldBeVal_TreatsThisReceiverAssignmentAsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class SendButton {
    private var scheduledSendListener: (() -> Unit)? = null

    fun setScheduledSendListener(listener: (() -> Unit)?) {
        this.scheduledSendListener = listener
    }

    fun fire() = scheduledSendListener?.invoke()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private property assigned through this receiver, got %d", len(findings))
	}
}

func TestVarCouldBeVal_TreatsUnqualifiedPropertyAssignmentAsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class FragmentLike {
    private var id = 0

    fun readArguments(args: Map<String, Int>) {
        id = args["id"] ?: 0
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private property assigned by name, got %d", len(findings))
	}
}

func TestVarCouldBeVal_TreatsObjectNameQualifiedAssignmentAsReassignment(t *testing.T) {
	// An object member written through the object's own name
	// (`Deps.configured = true`) is a reassignment. The directly-assignable
	// LHS is `<ObjectName>.<member>`, not `this.<member>`, so it previously
	// slipped past the reassignment walk.
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
object Deps {
    private var configured = false
    fun configure() {
        Deps.configured = true
    }
    fun read() = configured
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for object member reassigned via object name, got %d", len(findings))
	}
}

func TestVarCouldBeVal_TreatsEnclosingClassQualifiedAssignmentAsReassignment(t *testing.T) {
	// A companion-object member written through the enclosing class name
	// (`ProgressService.title = ...`) from a sibling nested class is a
	// reassignment. The write lives outside the member's immediate
	// class_body, so the scan must cover the outermost enclosing type.
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class ProgressService {
    companion object {
        private var title: String = ""
    }
    class Controller {
        fun update(newTitle: String) {
            ProgressService.title = newTitle
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for companion member reassigned via enclosing class name, got %d", len(findings))
	}
}

func TestVarCouldBeVal_QualifiedReadIsNotReassignment(t *testing.T) {
	// A qualified READ of an object member (`Config.enabled`, no assignment)
	// must not be mistaken for a reassignment — the never-reassigned member
	// still flags.
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
object Config {
    private var enabled = false
    fun read() = Config.enabled
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for object member that is only read through a qualified access")
	}
}

func TestVarCouldBeVal_TreatsLambdaPropertyAssignmentAsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class FragmentLike {
    private var id = 0

    fun readArguments(args: Map<String, Int>?) {
        args?.let {
            id = it["id"] ?: 0
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private property assigned inside lambda, got %d", len(findings))
	}
}

func TestVarCouldBeVal_TreatsSafeCallLambdaPropertyAssignmentAsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Args {
    fun getInt(key: String, default: Int): Int = default
}
class FragmentLike {
    val arguments: Args? = null
    private var id = 0

    fun onCreate() {
        val arguments = arguments
        arguments?.let {
            id = arguments.getInt("id", 0)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private property assigned inside safe-call lambda, got %d", len(findings))
	}
}

func TestVarCouldBeVal_SkipsMalformedClassBodyScope(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Holder {
    val child: Holder? = null
    var tags: String = ""
    fun get(): Holder? = this
}
class FragmentLike {
    private var id = 0

    fun update(holder: Holder?) {
        holder?.get()?.child?.tags = "updated"
    }
}
`)
	// The assignment through a call chain now parses, so the class body is
	// intact and the never-reassigned private `id` is reported. The assignment
	// to `tags` targets another object and must not count against `id`.
	if len(findings) != 1 {
		t.Fatalf("expected one finding for private var id, got %d", len(findings))
	}
}

func TestVarCouldBeVal_SkipsDelegationSpecifierParseShape(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
interface Scope
private class DelegatingScope(inner: Scope) : Scope by inner {
    var thread = Any()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for property parsed under delegation specifier, got %d", len(findings))
	}
}

func TestVarCouldBeVal_SkipsMemberPropertiesInMisparsedClassBody(t *testing.T) {
	// tree-sitter-kotlin cannot attach a primary constructor that starts on
	// the line after the class name (`class X\ninternal constructor(...)` or
	// `class X\n@JvmOverloads\nconstructor(...)`), so the class body is parsed
	// as a function or lambda body and its member properties look local. A
	// local variable can never carry a visibility modifier, so such a
	// property is a member written elsewhere through a receiver.
	cases := map[string]string{
		"constructor on next line": `
package test
class Frame
class Server {
    class Client
    internal constructor(
        internal val id: Int,
    ) {
        internal var pendingCount = 0
    }

    fun enqueue(client: Client) {
        client.pendingCount++
    }
}
`,
		"annotated constructor on next line": `
package test
class Checker
@JvmOverloads
constructor(
  private val getenv: (String) -> String? = System::getenv,
) : Runnable {
  private var sleeper: (Long) -> Unit = { ms -> Thread.sleep(ms) }

  internal constructor(
    getenv: (String) -> String?,
    sleeper: (Long) -> Unit,
  ) : this(getenv) {
    this.sleeper = sleeper
  }

  override fun run() = sleeper(1L)
}
`,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := runRuleByName(t, "VarCouldBeVal", code); len(findings) != 0 {
				t.Fatalf("expected no findings for member property in misparsed class body, got %d: %v", len(findings), findings)
			}
		})
	}
}

func TestVarCouldBeVal_TreatsOuterClassWriteToCompanionMemberAsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Ime {
    fun onCreate() {
        instance = this
    }

    fun onDestroy() {
        if (instance === this) instance = null
    }

    companion object {
        @Volatile private var instance: Ime? = null

        fun current(): Ime? = instance
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for companion member assigned from the enclosing class, got %d", len(findings))
	}
}

func TestVarCouldBeVal_OuterLocalShadowingCompanionMemberIsNotAReassignment(t *testing.T) {
	// Kotlin resolves a local variable or an outer-class member before an
	// implicit companion member, so writes to those do not reassign the
	// companion property of the same name.
	cases := map[string]string{
		"local var": `
package test
class Counter {
    fun tick(): Int {
        var count = 0
        count++
        return count
    }

    companion object {
        private var count = 0
        fun read(): Int = count
    }
}
`,
		"outer class member": `
package test
class Counter {
    private var count = 0

    fun tick() {
        count++
    }

    companion object {
        private var count = 0
        fun read(): Int = count
    }
}
`,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "VarCouldBeVal", code)
			flagged := false
			for _, f := range findings {
				if strings.Contains(f.Message, "Variable 'count'") {
					flagged = true
				}
			}
			if !flagged {
				t.Fatalf("expected the companion 'var count' to be flagged, got %v", findings)
			}
		})
	}
}

func TestVarCouldBeVal_CompanionMemberOnlyReadByEnclosingClassIsFlagged(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Ime {
    fun read(): Ime? = instance

    companion object {
        private var instance: Ime? = null
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for companion member that is never reassigned, got %d", len(findings))
	}
}

func TestVarCouldBeVal_TreatsPrefixIncrementInsideComparisonAsReassignment(t *testing.T) {
	// tree-sitter-kotlin parses `++count <= max` as `++(count <= max)`; the
	// increment target is the leftmost operand of the binary expression.
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
fun guard(nodes: List<Int>, max: Int) {
    var count = 0
    var other = 0
    fun visit(node: Int) {
        require(++count <= max) { "limit exceeded" }
        if (--other + node > max) return
    }
    nodes.forEach { visit(it) }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for vars mutated by prefix increment inside a binary expression, got %d: %v", len(findings), findings)
	}
}

func TestVarCouldBeVal_PrefixIncrementOfOtherOperandIsNotReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
fun guard(max: Int) {
    var count = 0
    var limit = max
    if (++count <= limit) println(limit)
}
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "limit") {
		t.Fatalf("expected exactly one finding for the right-hand operand var, got %v", findings)
	}
}

func TestVarCouldBeVal_HonorsIgnoreLateinitVar(t *testing.T) {
	// IgnoreLateinitVar was previously a dead config — exposed in
	// metadata but never consulted. The check unconditionally skipped
	// `lateinit var` declarations. Wire the field so the user can
	// opt OUT of the skip.
	var rule *rules.VarCouldBeValRule
	for _, candidate := range api.Registry {
		if candidate.ID == "VarCouldBeVal" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.VarCouldBeValRule)
			if !ok {
				t.Fatalf("expected VarCouldBeValRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("VarCouldBeVal rule not registered")
	}
	original := rule.IgnoreLateinitVar
	defer func() { rule.IgnoreLateinitVar = original }()

	// Pathological-but-syntactically-valid: a lateinit var that's never
	// assigned anywhere in the visible scope. Production code would
	// crash at first read, but the rule's reassignment analysis just
	// sees zero assignments and would therefore suggest `val` if it
	// weren't for the lateinit skip.
	code := `package test
class Foo {
    private lateinit var name: String
    fun read() = name
}`
	// Default (false, as in detekt): the reassignment analysis reports the lateinit var.
	if findings := runRuleByName(t, "VarCouldBeVal", code); len(findings) == 0 {
		t.Fatal("expected finding under the default IgnoreLateinitVar=false for never-reassigned lateinit var")
	}

	rule.IgnoreLateinitVar = true

	// With the flag on, the lateinit skip bypasses the reassignment analysis.
	if findings := runRuleByName(t, "VarCouldBeVal", code); len(findings) != 0 {
		t.Fatalf("expected no findings under IgnoreLateinitVar=true, got %d", len(findings))
	}
}

func TestVarCouldBeVal_ShippedDefaultConfigReportsLateinitVar(t *testing.T) {
	cfg, err := config.LoadAndMerge("", filepath.Join("..", "..", "config", "default-krit.yml"))
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}

	var rule *rules.VarCouldBeValRule
	for _, candidate := range api.Registry {
		if candidate.ID == "VarCouldBeVal" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.VarCouldBeValRule)
			if !ok {
				t.Fatalf("expected VarCouldBeValRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("VarCouldBeVal rule not registered")
	}
	original := rule.IgnoreLateinitVar
	defer func() { rule.IgnoreLateinitVar = original }()

	api.ApplyConfig(rule, rule.Meta(), rules.NewConfigAdapter(cfg))
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Foo
class Holder {
    internal companion object {
        private lateinit var x: Foo
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected a finding for the lateinit var under shipped defaults (detekt parity), got %d", len(findings))
	}

	if findings := runRuleByName(t, "VarCouldBeVal", `
package test
fun example() {
    var y = 1
    println(y)
}`); len(findings) == 0 {
		t.Fatal("expected finding for never-reassigned var under shipped defaults")
	}
}

func TestVarCouldBeVal_DoesNotRequireTypeContext(t *testing.T) {
	for _, rule := range api.Registry {
		if rule.ID == "VarCouldBeVal" {
			if rule.Needs.Has(api.NeedsResolver) || rule.Needs.Has(api.NeedsOracle) ||
				rule.Needs.Has(api.NeedsParsedFiles) || rule.Needs.Has(api.NeedsCrossFile) {
				t.Fatalf("VarCouldBeVal should stay AST-only; got needs %v", rule.Needs)
			}
			return
		}
	}
	t.Fatal("VarCouldBeVal rule not found")
}

func BenchmarkVarCouldBeValSharedScope(b *testing.B) {
	var src strings.Builder
	src.WriteString("package test\nfun example() {\n")
	for i := 0; i < 400; i++ {
		src.WriteString("    var value")
		src.WriteString(string(rune('A' + (i % 26))))
		src.WriteString("_")
		src.WriteString(strings.Repeat("x", i/26))
		src.WriteString(" = ")
		src.WriteString("0\n")
	}
	for i := 0; i < 100; i++ {
		src.WriteString("    value")
		src.WriteString(string(rune('A' + (i % 26))))
		src.WriteString("_")
		src.WriteString(strings.Repeat("x", i/26))
		src.WriteString(" = ")
		src.WriteString("1\n")
	}
	src.WriteString("}\n")

	path := filepath.Join(b.TempDir(), "bench.kt")
	if err := os.WriteFile(path, []byte(src.String()), 0644); err != nil {
		b.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		b.Fatal(err)
	}

	var target *api.Rule
	for _, r := range api.Registry {
		if r.ID == "VarCouldBeVal" {
			target = r
			break
		}
	}
	if target == nil {
		b.Fatal("VarCouldBeVal rule not found")
	}

	dispatcher := rules.NewDispatcher([]*api.Rule{target}, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dispatcher.Run(file)
	}
}

// --- MayBeConstant ---

func TestMayBeConstant_Positive(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test
val MAX_COUNT = 100
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for top-level val with constant initializer")
	}
}

func TestMayBeConstant_Negative(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test
const val MAX_COUNT = 100
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for const val, got %d", len(findings))
	}
}

func TestMayBeConstant_SameFileReferenceAndBinaryPositive(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test
val BASE = 40
val MAX_COUNT = BASE + 2
`)
	if len(findings) < 2 {
		t.Fatalf("expected findings for literal and same-file constant expression, got %d", len(findings))
	}
}

func TestMayBeConstant_NullNegative(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test
val NOTHING = null
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for null initializer, got %d", len(findings))
	}
}

// --- ModifierOrder ---

func TestModifierOrder_Positive(t *testing.T) {
	findings := runRuleByName(t, "ModifierOrder", `
package test
override public fun toString(): String = "test"
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for modifiers in wrong order (override before public)")
	}
}

func TestModifierOrder_Negative(t *testing.T) {
	findings := runRuleByName(t, "ModifierOrder", `
package test
public override fun toString(): String = "test"
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for correctly ordered modifiers, got %d", len(findings))
	}
}

// --- FunctionOnlyReturningConstant ---

func TestFunctionOnlyReturningConstant_Positive(t *testing.T) {
	findings := runRuleByName(t, "FunctionOnlyReturningConstant", `
package test
fun getAnswer(): Int {
    return 42
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for function only returning a constant")
	}
}

func TestFunctionOnlyReturningConstant_SkipsTestsAndErrorRecoveredBodies(t *testing.T) {
	findings := runRuleByName(t, "FunctionOnlyReturningConstant", `
package test

class FooTest {
    @Test
    fun `+"`reports the name`"+`() {
        setUp().storedName = "archive (1).zip"
        register(ROW, Stream())

        val export = exporter().export(range())

        assertThat(export.path).isEqualTo("/downloads/archive (1).zip")
    }

    @Test
    fun constantTest() = 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for test functions, got %d", len(findings))
	}
	findings = runRuleByName(t, "FunctionOnlyReturningConstant", `
package test

class Holder {
    fun run() {
        setUp().storedName = "x"
        register(ROW)
        check(value())
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a multi-statement Unit body, got %d", len(findings))
	}
}

func TestFunctionOnlyReturningConstant_Negative(t *testing.T) {
	findings := runRuleByName(t, "FunctionOnlyReturningConstant", `
package test
fun compute(x: Int): Int {
    return x * 2
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for function returning computed value, got %d", len(findings))
	}
}

func TestFunctionOnlyReturningConstant_HonorsIgnoreOverridableFunction(t *testing.T) {
	// IgnoreOverridableFunction was previously a dead config — exposed
	// in metadata but never consulted. Default true preserves krit's prior
	// behavior of skipping override/open/abstract.
	// Setting it to false makes those functions checkable.
	var rule *rules.FunctionOnlyReturningConstantRule
	for _, candidate := range api.Registry {
		if candidate.ID == "FunctionOnlyReturningConstant" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.FunctionOnlyReturningConstantRule)
			if !ok {
				t.Fatalf("expected FunctionOnlyReturningConstantRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("FunctionOnlyReturningConstant rule not registered")
	}
	original := rule.IgnoreOverridableFunction
	defer func() { rule.IgnoreOverridableFunction = original }()

	overrideCode := `package test
open class Base {
    open fun getAnswer(): Int = 41
}
class Sub : Base() {
    override fun getAnswer(): Int = 42
}
`
	// Default (true): override is skipped → no finding on Sub.getAnswer.
	if findings := runRuleByName(t, "FunctionOnlyReturningConstant", overrideCode); len(findings) > 0 {
		// Findings on Base's `open fun` are also expected to be suppressed
		// under the default — assert it's actually 0.
		t.Fatalf("expected no findings under IgnoreOverridableFunction=true, got %d", len(findings))
	}

	rule.IgnoreOverridableFunction = false

	// Flip to false: override/open are now checkable. Both Base and Sub fire.
	if findings := runRuleByName(t, "FunctionOnlyReturningConstant", overrideCode); len(findings) == 0 {
		t.Fatal("expected findings for override/open functions when IgnoreOverridableFunction=false")
	}
}

func TestFunctionOnlyReturningConstant_HonorsIgnoreActualFunction(t *testing.T) {
	// IgnoreActualFunction default true skips `actual fun` declarations.
	// Setting it to false makes them checkable.
	var rule *rules.FunctionOnlyReturningConstantRule
	for _, candidate := range api.Registry {
		if candidate.ID == "FunctionOnlyReturningConstant" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.FunctionOnlyReturningConstantRule)
			if !ok {
				t.Fatalf("expected FunctionOnlyReturningConstantRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("FunctionOnlyReturningConstant rule not registered")
	}
	original := rule.IgnoreActualFunction
	defer func() { rule.IgnoreActualFunction = original }()

	actualCode := `package test
actual fun getAnswer(): Int = 42
`
	// Default (true): actual is skipped.
	if findings := runRuleByName(t, "FunctionOnlyReturningConstant", actualCode); len(findings) != 0 {
		t.Fatalf("expected no findings under IgnoreActualFunction=true, got %d", len(findings))
	}

	rule.IgnoreActualFunction = false

	// Flip to false: actual fun is now checkable.
	if findings := runRuleByName(t, "FunctionOnlyReturningConstant", actualCode); len(findings) == 0 {
		t.Fatal("expected finding for actual fun when IgnoreActualFunction=false")
	}
}

// --- LoopWithTooManyJumpStatements ---

func TestLoopWithTooManyJumpStatements_Positive(t *testing.T) {
	findings := runRuleByName(t, "LoopWithTooManyJumpStatements", `
package test
fun process(items: List<Int>) {
    for (item in items) {
        if (item < 0) continue
        if (item > 100) break
        println(item)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for loop with 2 jump statements (max 1)")
	}
}

func TestLoopWithTooManyJumpStatements_MessageMatchesDetekt(t *testing.T) {
	findings := runRuleByName(t, "LoopWithTooManyJumpStatements", `
package test
fun process(items: List<Int>): Int {
    for (item in items) {
        if (item < 0) continue
        if (item > 100) break
        if (item == 50) return item
    }
    return 0
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	want := "The loop contains more than one break or continue statement. The code should be refactored to increase readability."
	if findings[0].Message != want {
		t.Fatalf("message = %q, want %q", findings[0].Message, want)
	}
}

func TestLoopWithTooManyJumpStatements_ReturnNotCounted(t *testing.T) {
	findings := runRuleByName(t, "LoopWithTooManyJumpStatements", `
package test
fun process(items: List<Int>): Int {
    for (item in items) {
        if (item < 0) continue
        if (item == 50) return item
        if (item == 60) return -item
    }
    return 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestLoopWithTooManyJumpStatements_Negative(t *testing.T) {
	findings := runRuleByName(t, "LoopWithTooManyJumpStatements", `
package test
fun process(items: List<Int>) {
    for (item in items) {
        if (item < 0) continue
        println(item)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for loop with 1 jump (within limit), got %d", len(findings))
	}
}

func TestLoopWithTooManyJumpStatements_CompatibleDefaults(t *testing.T) {
	var rule *rules.LoopWithTooManyJumpStatementsRule
	var meta api.RuleDescriptor
	for _, candidate := range api.Registry {
		if candidate.ID != "LoopWithTooManyJumpStatements" {
			continue
		}
		var ok bool
		rule, ok = candidate.Implementation.(*rules.LoopWithTooManyJumpStatementsRule)
		if !ok {
			t.Fatalf("expected LoopWithTooManyJumpStatementsRule, got %T", candidate.Implementation)
		}
		metaProvider, ok := candidate.Implementation.(api.MetaProvider)
		if !ok {
			t.Fatal("expected LoopWithTooManyJumpStatementsRule to provide metadata")
		}
		meta = metaProvider.Meta()
		break
	}
	if rule == nil {
		t.Fatal("LoopWithTooManyJumpStatements rule not registered")
	}
	if rule.MaxJumps != 1 {
		t.Fatalf("expected MaxJumps default 1, got %d", rule.MaxJumps)
	}
	for _, opt := range meta.Options {
		if opt.Name == "maxJumpCount" {
			if opt.Default != 1 {
				t.Fatalf("expected maxJumpCount metadata default 1, got %v", opt.Default)
			}
			return
		}
	}
	t.Fatal("maxJumpCount option not found")
}

// --- ExplicitItLambdaParameter ---

func TestExplicitItLambdaParameter_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExplicitItLambdaParameter", `
package test
fun example() {
    listOf(1, 2, 3).map { it -> it * 2 }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for explicit 'it' lambda parameter")
	}
}

func TestExplicitItLambdaParameter_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExplicitItLambdaParameter", `
package test
fun example() {
    listOf(1, 2, 3).map { it * 2 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for implicit 'it', got %d", len(findings))
	}
}

// --- ExplicitItLambdaMultipleParameters ---

func TestExplicitItLambdaMultipleParameters_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExplicitItLambdaMultipleParameters", `
package test
fun example() {
    mapOf(1 to "a").forEach { it, value -> println(value) }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for 'it' used as parameter name in multi-param lambda")
	}
}

func TestExplicitItLambdaMultipleParameters_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExplicitItLambdaMultipleParameters", `
package test
fun example() {
    mapOf(1 to "a").forEach { key, value -> println(value) }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for properly named multi-param lambda, got %d", len(findings))
	}
}

// --- MagicNumber (from style_forbidden.go) ---

func TestMagicNumber_Expr_Positive(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun compute(x: Int): Int {
    return x * 42
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for magic number 42")
	}
}

func TestMagicNumber_Expr_Negative(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun compute(x: Int): Int {
    return x * 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for allowed number 1, got %d", len(findings))
	}
}

// --- WildcardImport (from style_forbidden.go) ---

func TestWildcard_Positive(t *testing.T) {
	findings := runRuleByName(t, "WildcardImport", `
package test
import kotlin.collections.*
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for wildcard import")
	}
}

func TestWildcard_Negative(t *testing.T) {
	findings := runRuleByName(t, "WildcardImport", `
package test
import kotlin.collections.List
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for explicit import, got %d", len(findings))
	}
}

// --- UnusedParameter (from style_unused.go) ---

func TestUnusedParam_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test
fun greet(name: String, unused: Int) {
    println(name)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused parameter")
	}
}

func TestUnusedParam_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test
fun greet(name: String) {
    println(name)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all parameters are used, got %d", len(findings))
	}
}

// --- MaxLineLength (from style_format.go) ---

func TestMaxLine_Positive(t *testing.T) {
	// 121+ character line (excluding import/package which are skipped by default)
	findings := runRuleByName(t, "MaxLineLength", `package test
val result = aaaaaaaaa + bbbbbbbbb + ccccccccc + ddddddddd + eeeeeeeee + fffffffff + ggggggggg + hhhhhhhhh + iiiiiiiii + jjjjjjjjj
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for line exceeding 120 characters")
	}
}

func TestMaxLine_Negative(t *testing.T) {
	findings := runRuleByName(t, "MaxLineLength", `package test
val x = "short line"
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for short lines, got %d", len(findings))
	}
}

func TestVarCouldBeVal_ThisAssignmentInLambdaAndOtherFunctionCounts(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Holder {
    private var title: String = ""
    private var count: Int = 0
    fun update(values: List<Int>) {
        values.forEach { this.count = it }
    }
    private fun rename(value: String) {
        this.title = value
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestVarCouldBeVal_PublicMemberVarIsNotReported(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Holder {
    var title: String = ""
    internal var count: Int = 0
    protected var flag: Boolean = false
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-private vars, got %v", findings)
	}
}

func TestVarCouldBeVal_PrivateMemberVarWithoutWritesReported(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Holder {
    private var title: String = ""
    private var other: String = ""
    fun rename(value: String) {
        this.other = value
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for the unwritten private var, got %v", findings)
	}
}

func TestMayBeConstant_SkipsOverrideOpenAndLocals(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test

open class Base {
    open val limit = 2
    companion object {
        override val name = "base"
    }
}

fun compute(): Int {
    val local = 6
    return local
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMayBeConstant_FlagsObjectAndCompanionMembers(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test

object Defaults {
    val retries = 3
}

class Holder {
    companion object {
        val label = "holder"
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestReturnCountAndThrowsCount_HonourSuppressOnFunctionAndClass(t *testing.T) {
	const body = `
    fun f(x: Int): Int {
        if (x == 1) return 1
        if (x == 2) return 2
        if (x == 3) return 3
        return 4
    }
`
	suppressed := `
package test
@Suppress("ReturnCount")
class A {` + body + `}
class B {
    @Suppress("ReturnCount")` + body + `}
`
	if findings := runRuleByName(t, "ReturnCount", suppressed); len(findings) != 0 {
		t.Fatalf("expected suppressed ReturnCount findings to be dropped, got %d", len(findings))
	}
	if findings := runRuleByName(t, "ReturnCount", "package test\nclass C {"+body+"}\n"); len(findings) != 1 {
		t.Fatalf("expected 1 unsuppressed ReturnCount finding, got %d", len(findings))
	}
	const throws = `
    fun f(x: Int): Int {
        if (x == 1) throw IllegalStateException()
        if (x == 2) throw IllegalStateException()
        if (x == 3) throw IllegalStateException()
        return 4
    }
`
	if findings := runRuleByName(t, "ThrowsCount", "package test\n@Suppress(\"ThrowsCount\")\nclass A {"+throws+"}\n"); len(findings) != 0 {
		t.Fatalf("expected suppressed ThrowsCount findings to be dropped, got %d", len(findings))
	}
	if findings := runRuleByName(t, "ThrowsCount", "package test\nclass C {"+throws+"}\n"); len(findings) != 1 {
		t.Fatalf("expected 1 unsuppressed ThrowsCount finding, got %d", len(findings))
	}
}

func TestFunctionOnlyReturningConstant_HonorsSuppress(t *testing.T) {
	findings := runRuleByName(t, "FunctionOnlyReturningConstant", `
package test

class Config {
    @Suppress("FunctionOnlyReturningConstant")
    fun name() = "config"

    @Suppress("unused", "FunctionOnlyReturningConstant")
    fun version(): Int {
        return 3
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected suppressed findings, got %d", len(findings))
	}
	findings = runRuleByName(t, "FunctionOnlyReturningConstant", `
package test

class Config {
    fun name() = "config"
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding without @Suppress, got %d", len(findings))
	}
}

func TestVarCouldBeVal_SecondaryConstructorThisAssignmentIsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Counter {
    private var count = 0

    constructor(start: Int) {
        this.count = start
    }

    fun read(): Int = count
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when a secondary constructor assigns this.count, got %d: %v", len(findings), findings)
	}
}

func TestVarCouldBeVal_SecondaryConstructorParameterShadowIsNotReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Counter {
    private var count = 0

    constructor(count: Int) {
        println(count)
    }

    fun read(): Int = count
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected the never-written property to be reported, got %d: %v", len(findings), findings)
	}
}

func TestSafeCast_ImplicitReceiverPropertyIgnoredAndMessageSaysRedundant(t *testing.T) {
	unresolved := runRedundantSafeCast(t, `
package test
fun a() {
    produceState {
        if (value is Loaded) {
            use((value as Loaded).items)
        }
    }
}
`)
	if len(unresolved) != 0 {
		t.Fatalf("a subject not declared in the file cannot be proven stable, got %d", len(unresolved))
	}
	stable := runRedundantSafeCast(t, `
package test
fun b(items: List<Any>) {
    items.forEach { item ->
        if (item is String) {
            println((item as String).length)
        }
    }
}
`)
	if len(stable) != 1 {
		t.Fatalf("expected the lambda parameter to be flagged, got %d", len(stable))
	}
	if !strings.Contains(stable[0].Message, "Redundant cast after is-check") || strings.Contains(stable[0].Message, "as?") {
		t.Fatalf("unexpected message: %s", stable[0].Message)
	}
}

func TestSafeCast_MessageStartsWithRedundantCast(t *testing.T) {
	findings := runRedundantSafeCast(t, `
package test
fun f(x: Any) {
    if (x is String) {
        val s = x as String
        println(s)
    }
}
`)
	if len(findings) != 1 || !strings.HasPrefix(findings[0].Message, "Redundant cast") {
		t.Fatalf("expected one finding starting with 'Redundant cast', got %v", findings)
	}
}

func TestVarCouldBeVal_NullInitializerNeverAssignedMessage(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Holder {
    private var task: Task? = null
    private var count: Int = 0

    fun run() {
        task?.cancel()
        println(count)
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("both vars stay reported, got %d: %v", len(findings), findings)
	}
	for _, f := range findings {
		switch f.Message {
		case "Variable 'task' could be val.", "Variable 'count' could be val.":
		default:
			t.Fatalf("unexpected finding %q", f.Message)
		}
	}
}

func TestCollapsibleIfStatements_FixKeepsPrecedenceOfOrConditions(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun h(a: Boolean, b: Boolean, c: Boolean, d: Boolean) {
    if (a || b) {
        if (c) {
            println("x")
        }
    }
    if ((a) || (b)) {
        if ((c) && (d)) {
            println("z")
        }
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	for i, want := range []string{"if ((a || b) && c) {", "if (((a) || (b)) && (c) && (d)) {"} {
		if findings[i].Fix == nil || !strings.HasPrefix(findings[i].Fix.Replacement, want) {
			t.Errorf("finding %d: fix = %+v, want prefix %q", i, findings[i].Fix, want)
		}
	}
}

func TestMayBeConstant_SkipsAccessorsDelegatesAndNonConstTypes(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test

val withGetter = 1
    get() = field + 1
val nullable: String? = "a"
val customType: Foo = 1
actual val actualVal = 1
val plain = 1
val typed: Long = 1L
`)
	if len(findings) != 2 {
		t.Fatalf("expected only plain and typed, got %d: %v", len(findings), findings)
	}
}

func TestModifierOrder_FixKeepsAnnotationsBetweenModifiers(t *testing.T) {
	src := `package test
class A {
    @Suppress("X") override public fun r() {}
    @Ann(1)
    suspend private fun t() {}
}
`
	findings := runRuleByName(t, "ModifierOrder", src)
	if len(findings) != 2 {
		t.Fatalf("expected two findings, got %d", len(findings))
	}
	apply := func(f scanner.Finding) string { return src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:] }
	if got := apply(findings[0]); !strings.Contains(got, `@Suppress("X") public override fun r()`) {
		t.Errorf("annotation lost or modifiers unsorted: %s", got)
	}
	if got := apply(findings[1]); !strings.Contains(got, "@Ann(1)\n    private suspend fun t()") {
		t.Errorf("annotation lost or modifiers unsorted: %s", got)
	}
}

func TestVarCouldBeVal_PrivateMemberWrittenThroughAnotherInstanceOrVolatile(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class A {
    private var viaOther = 0
    private var untouched = 0
    @Volatile private var flag = false
    val other = A()
    fun f() { other.viaOther = 1 }
}
`)
	if len(findings) != 1 || findings[0].Line != 5 {
		t.Fatalf("only the untouched private var is a candidate, got %v", findings)
	}
}

func TestSafeCast_IsCheckReturningValueOrNull(t *testing.T) {
	src := `package test
fun f(x: Any, y: Any?) {
    val a = if (x is String) x else null
    val b = if (x !is String) null else x
    val c = if (x is String) x else "d"
    var m: Any = x
    val d = if (m is String) m else null
}
`
	findings := runRuleByName(t, "SafeCast", src)
	if len(findings) != 3 || findings[0].Line != 3 || findings[1].Line != 4 || findings[2].Line != 7 {
		t.Fatalf("expected findings on lines 3, 4 and 7, got %v", findings)
	}
	fix := findings[0].Fix
	if fix == nil || fix.Replacement != "x as? String" {
		t.Fatalf("expected an 'x as? String' fix, got %+v", fix)
	}
}

func TestCollapsibleIfStatements_FixKeepsOperandPrecedence(t *testing.T) {
	src := `package test
fun t(a: Boolean, b: Boolean, c: Boolean) {
    if (a || b) {
        if (c) {
            println()
        }
    }
    if ((a) || (b)) {
        if ((c)) println()
    }
}
`
	findings := runRuleByName(t, "CollapsibleIfStatements", src)
	if len(findings) != 2 {
		t.Fatalf("expected two findings, got %d", len(findings))
	}
	if got := findings[0].Fix.Replacement; !strings.HasPrefix(got, "if ((a || b) && c)") {
		t.Errorf("disjunction must stay parenthesized: %s", got)
	}
	if got := findings[1].Fix.Replacement; !strings.HasPrefix(got, "if (((a) || (b)) && c)") {
		t.Errorf("unexpected merge: %s", got)
	}
}

// --- MayBeConstant (detekt MayBeConst) ---

func TestMayBeConstant_DetektSemantics(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test

val topLevel = 1
val greeting = "hello"
val interpolated = "a${topLevel}"
val sum = 1 + 2 * 3
val wrapped = (1 + 2)
var mutable = 1
const val alreadyConst = 1
@JvmField val jvmField = 1
val withGetter get() = 1
val computed = listOf(1)

class Holder {
    val member = 1
    companion object {
        val inCompanion = 1
        override val overridden = 1
    }
}

object Settings {
    val inObject = 5L
}

fun f() {
    val local = 1
}
`)
	got := map[string]bool{}
	for _, f := range findings {
		got[f.Message] = true
	}
	for _, name := range []string{"topLevel", "greeting", "sum", "wrapped", "inCompanion", "inObject"} {
		if !got[name+" can be a `const val`."] {
			t.Errorf("expected a finding for %s, got %v", name, got)
		}
	}
	if len(findings) != 6 {
		t.Errorf("expected exactly 6 findings, got %d: %v", len(findings), got)
	}
}

func TestLoopWithTooManyJumpStatements_CountsJumpsInLambdasButNotNestedLoops(t *testing.T) {
	findings := runRuleByName(t, "LoopWithTooManyJumpStatements", `
package test
fun process(items: List<List<Int>>) {
    for (row in items) {
        row.forEach {
            if (it < 0) continue
            if (it > 100) break
        }
    }
    for (row in items) {
        for (item in row) {
            if (item < 0) continue
        }
        if (row.isEmpty()) break
    }
}
`)
	if len(findings) != 1 || findings[0].Line != 4 {
		t.Fatalf("expected only the first loop (non-local jumps in a lambda) to be flagged, got %v", findings)
	}
}

func TestReturnCount_DetektMessageAndExcludedFunctionPatterns(t *testing.T) {
	findings := runRuleByName(t, "ReturnCount", `
package test
fun noncompliant(x: Int): Int {
    if (x < 0) return -1
    if (x == 0) return 0
    return 1
}
class Foo {
    override fun equals(other: Any?): Boolean {
        if (other == null) return false
        if (other === this) return true
        return false
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding (equals is excluded by default), got %v", findings)
	}
	want := "Function noncompliant has 3 return statements which exceeds the limit of 2."
	if findings[0].Message != want {
		t.Fatalf("message = %q, want %q", findings[0].Message, want)
	}
}

func TestReturnCount_ReportsTheFullCountWhenGuardClausesAreExcluded(t *testing.T) {
	findings := runRuleByName(t, "ReturnCount", `
package test
fun f(x: Int): Int {
    when (x) {
        1 -> return 10
        2 -> return 20
        3 -> return 30
    }
    if (x == 4) return 40
    return 50
}
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "has 5 return statements") {
		t.Fatalf("expected one finding counting 5 returns, got %v", findings)
	}
}
