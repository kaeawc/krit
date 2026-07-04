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

func TestEmptyFunctionBlock_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyFunctionBlock", `
package test;
class Example {
  void empty() {}
  void used() { System.out.println("ok"); }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java empty method finding, got %d", len(findings))
	}
}

// TestEmptyFunctionBlock_SkipsOverrideEmptyBodyByDefault verifies that empty
// override bodies are NOT flagged by default. An `override fun foo() {}` is
// almost always an intentional no-op required by a framework/interface
// contract (TextWatcher.afterTextChanged, AnimatorListener.onAnimationEnd,
// etc.), which made empty overrides the dominant false-positive source on
// real apps. The skip is driven by the AST `override` modifier node, not a
// text scan.
func TestEmptyFunctionBlock_SkipsOverrideEmptyBodyByDefault(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
package test
class Listener : AnimatorListener {
    override fun onAnimationEnd(a: Animator) {}
    override fun onAnimationStart(a: Animator) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for empty override bodies by default, got %d", len(findings))
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
func TestEmptyFunctionBlock_JavaOverrideAndCommentOnly(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyFunctionBlock", `
package test
class Example {
    @Override
    public void onEvent() {}

    public void documentedNoOp() {
        // intentionally no-op
    }

    public void forgotten() {}
}
`)
	if len(findings) != 1 {
		for _, f := range findings {
			t.Logf("  %s:%d %s", f.File, f.Line, f.Message)
		}
		t.Fatalf("expected 1 finding (only the non-override, non-commented empty method), got %d", len(findings))
	}
}

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

func TestEmptyCatchBlock_Java(t *testing.T) {
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
	if len(findings) == 0 {
		t.Fatal("expected Java empty catch finding")
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

func TestEmptyIfBlock_Java(t *testing.T) {
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
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java empty if finding, got %d", len(findings))
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

func TestEmptyElseBlock_Java(t *testing.T) {
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
	if len(findings) == 0 {
		t.Fatal("expected Java empty else finding")
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

func TestEmptyForBlock_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyForBlock", `
package test;
class Example {
  void run(int[] values) {
    for (int value : values) {}
    for (int i = 0; i < values.length; i++) { System.out.println(values[i]); }
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java empty for finding, got %d", len(findings))
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

func TestEmptyWhileBlock_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyWhileBlock", `
package test;
class Example {
  void run(boolean ok) {
    while (ok) {}
    while (!ok) { break; }
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java empty while finding, got %d", len(findings))
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

func TestEmptyTryBlock_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "EmptyTryBlock", `
package test;
class Example {
  void run() {
    try {} catch (Exception e) { handle(); }
  }
  void handle() {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected Java empty try finding")
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

func TestEmptyFinallyBlock_Java(t *testing.T) {
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
	if len(findings) == 0 {
		t.Fatal("expected Java empty finally finding")
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

func TestEmptyDoWhileBlock_Java(t *testing.T) {
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
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java empty do-while finding, got %d", len(findings))
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
	if rule.OracleCallTargets != nil || rule.OracleDeclarationNeeds != nil || rule.Oracle != nil {
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

// --- EmptySecondaryConstructor ---

func TestEmptySecondaryConstructor_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptySecondaryConstructor", `
class Foo(val x: Int) {
    constructor(x: Int, y: Int) : this(x) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty secondary constructor")
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
