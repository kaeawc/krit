package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// --- EmptyFunctionBlock ---

func TestEmptyFunctionBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
fun foo() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty function block")
	}
}

func TestEmptyFunctionBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
fun foo() {
    println("hello")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// TestEmptyFunctionBlock_NegativeCommentOnly is a lexical-negative
// regression for the bodyText scan that used to read
// `strings.HasPrefix(trimmedInner, "//") || strings.Contains(trimmedInner, "TODO")`.
// The replacement uses blockHasCommentFlat, which asks the AST whether
// a comment node lives inside the body. Both a line-comment marker and
// a block-comment marker must continue to suppress the finding, and
// the comment text containing the word "TODO" must not influence the
// outcome any differently than a comment without it.
func TestEmptyFunctionBlock_NegativeCommentOnly(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "line comment with TODO",
			src: `
fun foo() {
    // TODO: implement later
}
`,
		},
		{
			name: "line comment without TODO",
			src: `
fun foo() {
    // intentionally left as a placeholder
}
`,
		},
		{
			name: "block comment with TODO",
			src: `
fun foo() {
    /* TODO: implement later */
}
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "EmptyFunctionBlock", tc.src)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestEmptyFunctionBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyFunctionBlock", `
package test;
class Example {
  void empty() {}
  void used() { System.out.println("ok"); }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// detekt 1.23.8 reports an empty override unless ignoreOverridden is set, which defaults to false.
func TestEmptyFunctionBlock_ReportsOverrideEmptyBodyByDefault(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
package test
class Listener : AnimatorListener {
    override fun onAnimationEnd(a: Animator) {}
    override fun onAnimationStart(a: Animator) {}
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for empty override bodies by default, got %d", len(findings))
	}
}

// TestEmptyFunctionBlock_HonorsIgnoreOverriddenFalse verifies the wired field:
// when ignoreOverridden is set to false, override functions with empty bodies
// are flagged again. This is the aggressive opt-in mode that catches, e.g., an
// empty X509TrustManager.checkClientTrusted override that disables certificate
// checks.
func TestEmptyFunctionBlock_HonorsIgnoreOverriddenFalse(t *testing.T) {
	var rule *rules.EmptyFunctionBlockRule
	for _, candidate := range api.Registry {
		if candidate.ID == "EmptyFunctionBlock" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.EmptyFunctionBlockRule)
			if !ok {
				t.Fatalf("expected EmptyFunctionBlockRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("EmptyFunctionBlock rule not registered")
	}
	original := rule.IgnoreOverridden
	defer func() { rule.IgnoreOverridden = original }()

	rule.IgnoreOverridden = false

	findings := runRuleByName(t, "EmptyFunctionBlock", `
package test
class TrustNothing : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {}
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (one per empty override body) when ignoreOverridden=false, got %d", len(findings))
	}
}

// TestEmptyFunctionBlock_JavaOverrideAndCommentOnly locks the Java parity for
// the same two false positives: an `@Override` no-op and a body whose only
// content is a comment must not be flagged, while a genuinely-empty
// non-override method still is. The override is detected via the AST
// `marker_annotation` node and the comment via the `block`'s `line_comment`
// child — never a text scan.
// --- EmptyClassBlock ---

func TestEmptyClassBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyClassBlock", `
class Foo {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty class body")
	}
}

func TestEmptyClassBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyClassBlock", `
class Foo {
    val x = 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyClassBlock_Java(t *testing.T) {
	// Java requires `{}` on a class declaration — `class Empty` without a
	// body is a syntax error, so the rule's "remove the empty body" fix
	// can't apply. The rule is Kotlin-only.
	findings := runRuleByNameOnJava(t, "EmptyClassBlock", `
package test;
class Empty {}
class NonEmpty {
  int value;
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java empty class findings (rule is Kotlin-only), got %d", len(findings))
	}
}

func TestEmptyClassBlock_NegativeCommentAndAnonymousObject(t *testing.T) {
	findings := runRuleByName(t, "EmptyClassBlock", `
class Documented {
    // intentionally empty
}
class Block {
    /* reserved for later */
}
val anonymous = object { }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyClassBlock_ReportsInterfacesObjectsEnumsAndCompanions(t *testing.T) {
	findings := runRuleByName(t, "EmptyClassBlock", `
interface Marker {}
object Registry {}
enum class Mode {}
class Host { companion object {} }
`)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(findings))
	}
}

func TestEmptyBlocks_ReportOnTheBrace(t *testing.T) {
	// detekt reports an empty block where its `{` is, which differs from the keyword's line for multi-line signatures.
	findings := runRuleByName(t, "EmptyFunctionBlock", "package test\nfun f(\n    a: Int,\n) {}\n")
	if len(findings) != 1 || findings[0].Line != 4 || findings[0].Col != 3 {
		t.Fatalf("expected one finding on the brace at 4:3, got %+v", findings)
	}
	findings = runRuleByName(t, "EmptyIfBlock", "package test\nfun f(b: Boolean) {\n    if (b)\n    {\n    }\n}\n")
	if len(findings) != 1 || findings[0].Line != 4 {
		t.Fatalf("expected the if finding on line 4, got %+v", findings)
	}
}

// --- EmptyCatchBlock ---

func TestEmptyCatchBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyCatchBlock", `
fun foo() {
    try {
        doSomething()
    } catch (e: Exception) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty catch block")
	}
}

// TestEmptyCatchBlock_DoesNotEmitFix pins the autofix removal. The
// previous autofix inserted `// TODO: handle exception`, which is
// not a real fix — the catch still swallows the exception. The rule
// must continue to flag the violation but must never attach a Fix.
func TestEmptyCatchBlock_DoesNotEmitFix(t *testing.T) {
	findings := runRuleByName(t, "EmptyCatchBlock", `
fun foo() {
    try {
        doSomething()
    } catch (e: Exception) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty catch block")
	}
	for _, f := range findings {
		if f.Fix != nil {
			t.Fatalf("EmptyCatchBlock must not emit a Fix; got %+v", f.Fix)
		}
	}
}

func TestEmptyCatchBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyCatchBlock", `
fun foo() {
    try {
        doSomething()
    } catch (e: Exception) {
        e.printStackTrace()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyCatchBlock_NegativeCommentOnly(t *testing.T) {
	findings := runRuleByName(t, "EmptyCatchBlock", `
fun foo() {
    try {
        doSomething()
    } catch (e: Exception) {
        // intentionally ignored
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyCatchBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyCatchBlock", `
package test;
class Example {
  void run() {
    try {
      work();
    } catch (Exception e) {}
  }
  void work() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

func TestEmptyCatchBlock_JavaNegativeCommentOnly(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyCatchBlock", `
package test;
class Example {
  void run() {
    try {
      work();
    } catch (Exception e) {
      // intentionally ignored
    }
  }
  void work() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyCatchBlock_JavaNegativeBlockCommentOnly(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyCatchBlock", `
package test;
class Example {
  void run() {
    try {
      work();
    } catch (Exception e) {
      /* intentionally ignored */
    }
  }
  void work() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyIfBlock ---

func TestEmptyIfBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyIfBlock", `
fun foo() {
    val x = true
    if (x) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty if block")
	}
}

func TestEmptyIfBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyIfBlock", `
fun foo() {
    val x = true
    if (x) {
        println("yes")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyIfBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyIfBlock", `
package test;
class Example {
  void run(boolean ok) {
    if (ok) {}
    if (!ok) { work(); }
  }
  void work() { System.out.println("ok"); }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// --- EmptyElseBlock ---

func TestEmptyElseBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyElseBlock", `
fun foo() {
    val x = true
    if (x) {
        doSomething()
    } else {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty else block")
	}
}

func TestEmptyElseBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyElseBlock", `
fun foo() {
    val x = true
    if (x) {
        doSomething()
    } else {
        doOther()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyElseBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyElseBlock", `
package test;
class Example {
  void run(boolean ok) {
    if (ok) {
      work();
    } else {}
  }
  void work() { System.out.println("ok"); }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// --- EmptyWhenBlock ---

func TestEmptyWhenBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhenBlock", `
fun foo() {
    val x = 1
    when (x) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty when block")
	}
}

func TestEmptyWhenBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhenBlock", `
fun foo() {
    val x = 1
    when (x) {
        1 -> println("one")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyForBlock ---

func TestEmptyForBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyForBlock", `
fun foo() {
    val list = listOf(1, 2, 3)
    for (i in list) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty for block")
	}
}

func TestEmptyForBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyForBlock", `
fun foo() {
    val list = listOf(1, 2, 3)
    for (i in list) {
        println(i)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyForBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyForBlock", `
package test;
class Example {
  void run(int[] values) {
    for (int value : values) {}
    for (int i = 0; i < values.length; i++) { System.out.println(values[i]); }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// --- EmptyWhileBlock ---

func TestEmptyWhileBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhileBlock", `
fun foo() {
    while (true) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty while block")
	}
}

func TestEmptyWhileBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhileBlock", `
fun foo() {
    while (true) {
        Thread.sleep(100)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyWhileBlock_BracelessBodyIsNotEmpty(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhileBlock", `
fun foo(next: () -> Int) {
    var state = next()
    while (state < 3) state = next()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a brace-less body, got %d", len(findings))
	}
}

func TestEmptyWhileBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyWhileBlock", `
package test;
class Example {
  void run(boolean ok) {
    while (ok) {}
    while (!ok) { break; }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// --- EmptyTryBlock ---

func TestEmptyTryBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyTryBlock", `
fun foo() {
    try {} catch (e: Exception) {
        handle()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty try block")
	}
}

func TestEmptyTryBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyTryBlock", `
fun foo() {
    try {
        doSomething()
    } catch (e: Exception) {
        handle()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyTryBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyTryBlock", `
package test;
class Example {
  void run() {
    try {} catch (Exception e) { handle(); }
  }
  void handle() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// --- EmptyFinallyBlock ---

func TestEmptyFinallyBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyFinallyBlock", `
fun foo() {
    try {
        doSomething()
    } finally {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty finally block")
	}
}

func TestEmptyFinallyBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyFinallyBlock", `
fun foo() {
    try {
        doSomething()
    } finally {
        cleanup()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyFinallyBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyFinallyBlock", `
package test;
class Example {
  void run() {
    try {
      work();
    } finally {}
  }
  void work() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// --- EmptyInitBlock ---

func TestEmptyInitBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyInitBlock", `
class Foo {
    init {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty init block")
	}
}

func TestEmptyInitBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyInitBlock", `
class Foo {
    init {
        println("initialized")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyDoWhileBlock ---

func TestEmptyDoWhileBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyDoWhileBlock", `
fun foo() {
    do {} while (true)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty do-while block")
	}
}

func TestEmptyDoWhileBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyDoWhileBlock", `
fun foo() {
    do {
        Thread.sleep(100)
    } while (true)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyDoWhileBlock_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyDoWhileBlock", `
package test;
class Example {
  void run(boolean ok) {
    do {} while (ok);
    do { work(); } while (!ok);
  }
  void work() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// --- EmptyDefaultConstructor ---

func TestEmptyDefaultConstructor_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyDefaultConstructor", `
class Foo constructor() {
    val x = 1
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty default constructor")
	}
}

func TestEmptyDefaultConstructor_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyDefaultConstructor", `
class Foo(val name: String) {
    val x = 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyDefaultConstructor_NegativeSecondaryConstructorsAndModifiers(t *testing.T) {
	findings := runRuleByName(t, "EmptyDefaultConstructor", `
class Delegating constructor() {
    constructor(x: Int) : this()
}
class Hidden private constructor()
class Tagged @Marker constructor()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyDefaultConstructor_IgnoresDIAnnotatedConstructors(t *testing.T) {
	findings := runRuleByName(t, "EmptyDefaultConstructor", `
package test

import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.Inject

interface Service

@ContributesBinding(AppScope::class)
@Inject
class RealService() : Service

object AppScope
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for DI annotated empty constructor, got %d", len(findings))
	}
}

func TestEmptyDefaultConstructor_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["EmptyDefaultConstructor"]
	if rule == nil {
		t.Fatal("EmptyDefaultConstructor rule is not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("EmptyDefaultConstructor should remain AST-only, got needs %v", rule.Needs)
	}
	if rule.Oracle != nil {
		t.Fatal("EmptyDefaultConstructor should not declare oracle metadata")
	}
}

// --- EmptyKotlinFile ---

func TestEmptyKotlinFile_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyKotlinFile", `
package test
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty Kotlin file")
	}
}

func TestEmptyKotlinFile_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyKotlinFile", `
package test
fun hello() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyKotlinFile_NegativePlaceholderComment(t *testing.T) {
	for name, src := range map[string]string{
		"line":  "package test\n\n// Placeholder so the module has a source set.\n",
		"block": "package test\n\n/* This file is intentionally empty. */\n",
	} {
		t.Run(name, func(t *testing.T) {
			if findings := runRuleByName(t, "EmptyKotlinFile", src); len(findings) != 0 {
				t.Fatalf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestEmptyKotlinFile_PositiveUnrelatedComment(t *testing.T) {
	findings := runRuleByName(t, "EmptyKotlinFile", "package test\n\n// Copyright 2024 Example\n")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

// --- EmptySecondaryConstructor ---

func TestEmptySecondaryConstructor_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptySecondaryConstructor", `
class Foo {
    constructor() {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty secondary constructor")
	}
}

// detekt 1.23.8 reports an empty secondary constructor even when it delegates with this(...) or super(...).
func TestEmptySecondaryConstructor_ReportsDelegatingConstructorLikeDetekt(t *testing.T) {
	findings := runRuleByName(t, "EmptySecondaryConstructor", `
class Foo(val x: Int) {
    constructor(x: Int, y: Int) : this(x) {}
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected a finding for a delegating constructor with an empty body, got %d", len(findings))
	}
}

func TestEmptySecondaryConstructor_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptySecondaryConstructor", `
class Foo(val x: Int) {
    constructor(x: Int, y: Int) : this(x) {
        println("initialized with y=$y")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyFunctionBlock_SkipsExpressionBodyEndingInEmptyLambda(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
package test
fun build() = Widget { }
fun launch() = run {}
fun listener(): Runnable = Runnable {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestEmptyFunctionBlock_SkipsEmptyLambdaInsideRealBody(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
package test
fun register(host: Host) {
    host.onClick {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestEmptyFunctionBlock_FlagsEmptyBlockBody(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
package test
fun nothing() { }
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %v", findings)
	}
}

func TestEmptyIfBlock_KotlinCommentOrLambdaIsNotEmpty(t *testing.T) {
	findings := runRuleByName(t, "EmptyIfBlock", `
package test
fun f(c: Boolean, m: Modifier) {
    if (c) {
        // intentionally nothing
    }
    val x = if (c) m.foo {} else m
    if (c) m.foo {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyIfBlock_KotlinEmptyBracesStillFlagged(t *testing.T) {
	findings := runRuleByName(t, "EmptyIfBlock", `
package test
fun f(c: Boolean) {
    if (c) {
    } else {
        g()
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestEmptyElseBlock_CommentAndWhenElseAreNotEmpty(t *testing.T) {
	findings := runRuleByName(t, "EmptyElseBlock", `
package test
fun f(c: Boolean, x: Int) {
    if (c) {
        g()
    } else {
        // nothing to do
    }
    when (x) {
        1 -> g()
        else -> {}
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyForBlock_BodiesWithStatementsAreNotEmpty(t *testing.T) {
	findings := runRuleByName(t, "EmptyForBlock", `
package test
fun f(xs: List<Int>) {
    for (i in xs) g(i)
    for (i in xs) g(i) {}
    for (i in xs) { g(i) }
    for (i in xs) {
        // placeholder
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyForBlock_KotlinEmptyBracesFlagged(t *testing.T) {
	findings := runRuleByName(t, "EmptyForBlock", `
package test
fun f(xs: List<Int>) {
    for (i in xs) { }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestEmptyForBlock_JavaBracelessAndCommentBodiesAreNotEmpty(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyForBlock", `
class A {
    void f(int[] xs) {
        for (int x : xs) g(x);
        for (int x : xs) {
            // placeholder
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyWhileBlock_CommentOrSideEffectConditionIgnored(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhileBlock", `
package test
class Chan { fun awaitItem(): Int? = null }
fun f(c: Chan) {
    while (c.awaitItem() != null) {
    }
    while (true) {
        // spin until interrupted
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyWhileBlock_PlainFlagConditionStillFlagged(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhileBlock", `
package test
fun f(flag: Boolean) {
    while (flag) {
    }
}
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

// --- soft keyword used as an identifier ---

// A branch holding only a soft keyword identifier parses as a modifier with no declaration, so the
// tree has no statements node. The branch is still non-empty source.
func TestEmptyBlocks_SoftKeywordIdentifierBranchIsNotEmpty(t *testing.T) {
	for _, kw := range []string{"internal", "external", "open", "data", "inner", "value", "inline", "abstract", "final", "override", "lateinit", "tailrec", "operator", "infix", "suspend"} {
		src := `
fun foo(c: Boolean, ` + kw + `: String, other: String) {
    val v = if (c) { ` + kw + ` } else { ` + kw + ` }
    for (i in 0..1) { ` + kw + ` }
    v.length
}
`
		for _, rule := range []string{"EmptyIfBlock", "EmptyElseBlock", "EmptyForBlock"} {
			if got := runRuleByName(t, rule, src); len(got) != 0 {
				t.Errorf("%s with soft keyword %q: expected no findings, got %d", rule, kw, len(got))
			}
		}
	}
}

func TestEmptyBlocks_SoftKeywordStillFlagsTrulyEmpty(t *testing.T) {
	src := `
fun foo(c: Boolean) {
    if (c) { /* x */ } else { }
    if (c) {  } else { internal }
}
`
	if got := runRuleByName(t, "EmptyElseBlock", src); len(got) != 1 {
		t.Fatalf("expected 1 EmptyElseBlock finding, got %d", len(got))
	}
}

func TestEmptyBlocks_SoftKeywordIdentifierInTextBasedRules(t *testing.T) {
	src := `
class K(val internal: Int) {
    init { internal }
    fun f(open: Int, value: Int) { open }
    fun g(data: Int) {
        try { data } catch (e: Exception) { data } finally { data }
        while (data > 0) { data }
    }
}
`
	for _, rule := range []string{"EmptyFunctionBlock", "EmptyTryBlock", "EmptyCatchBlock", "EmptyFinallyBlock", "EmptyInitBlock", "EmptyWhileBlock"} {
		if got := runRuleByName(t, rule, src); len(got) != 0 {
			t.Errorf("%s: expected no findings, got %d", rule, len(got))
		}
	}
}

func TestEmptyBlocks_CommentOnlyBodiesAreNotEmpty(t *testing.T) {
	cases := map[string]string{
		"EmptyInitBlock":    "class A {\n    init {\n        // intentionally blank\n    }\n}\n",
		"EmptyFinallyBlock": "fun f() {\n    try { g() } finally { /* nothing */ }\n}\nfun g() {}\n",
		"EmptyTryBlock":     "fun f() {\n    try { /* nothing */ } catch (e: Exception) { g() }\n}\nfun g() {}\n",
		"EmptyWhenBlock":    "fun f(x: Int) {\n    when (x) { /* nothing */ }\n}\n",
		"EmptyDoWhileBlock": "fun f(x: Int) {\n    do { /* spin */ } while (x > 0)\n}\n",
	}
	for rule, src := range cases {
		if got := runRuleByName(t, rule, src); len(got) != 0 {
			t.Errorf("%s: want no findings for a comment-only body, got %d", rule, len(got))
		}
	}
}

func TestEmptyBlocks_BracesOutsideTheBodyAreIgnored(t *testing.T) {
	if got := runRuleByName(t, "EmptySecondaryConstructor", "class B {\n    constructor(f: () -> Unit = { })\n}\n"); len(got) != 0 {
		t.Errorf("secondary constructor without a body must not be reported, got %d", len(got))
	}
	if got := runRuleByName(t, "EmptySecondaryConstructor", "class B {\n    constructor(f: () -> Unit = { println() }) {}\n}\n"); len(got) != 1 {
		t.Errorf("empty secondary constructor body must be reported once, got %d", len(got))
	}
	if got := runRuleByName(t, "EmptyDoWhileBlock", "fun f(xs: List<Int>) {\n    do { } while (xs.any { it > 0 })\n}\n"); len(got) != 1 {
		t.Errorf("empty do-while with a lambda condition must be reported once, got %d", len(got))
	}
	if got := runRuleByName(t, "EmptyWhenBlock", "fun f() {\n    when (val y = run { 1 }) { }\n}\n"); len(got) != 1 {
		t.Errorf("empty when with a lambda subject must be reported once, got %d", len(got))
	}
}

// detekt reports an else-less if when a ';' directly follows it, even though no else exists.
func TestEmptyElseBlock_SemicolonAfterIf(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"unbraced then", "if (x > 1) println(); println()", 1},
		{"braced then", "if (x > 1) { println() }; println()", 1},
		{"space before semicolon", "if (x > 1) println() ; println()", 0},
		{"no semicolon", "if (x > 1) println()\n    println()", 0},
		{"else after semicolon", "if (x > 1) println(); else println()", 0},
		{"with else", "if (x > 1) println() else println(); println()", 0},
		{"expression use", "val y = if (x > 1) 1 else 2; println(y)", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nfun f(x: Int) {\n    " + tc.body + "\n}\n"
			if got := runRuleByName(t, "EmptyElseBlock", src); len(got) != tc.want {
				t.Fatalf("got %d findings, want %d: %v", len(got), tc.want, got)
			}
		})
	}
}
