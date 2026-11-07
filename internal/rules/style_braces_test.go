package rules

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// parseBracesInline writes code to a temp file and parses it. Used instead of
// the rules_test.parseInline helper since this file lives in package rules.
func parseBracesInline(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func parseBracesPath(t *testing.T, name, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// makeBracesIfV2Rule wraps a BracesOnIfStatementsRule in a native api.Rule,
// delegating to the (unexported) checkFlatNode implementation.
func makeBracesIfV2Rule(rule *BracesOnIfStatementsRule) *api.Rule {
	return &api.Rule{
		ID:         rule.RuleName,
		Category:   rule.RuleSetName,
		Sev:        api.Severity(rule.Sev),
		NodeTypes:  []string{"if_expression"},
		Confidence: rule.Confidence(),
		Check: func(ctx *api.Context) {
			rule.check(ctx)
		},
	}
}

// makeBracesWhenV2Rule wraps a BracesOnWhenStatementsRule in a native api.Rule.
func makeBracesWhenV2Rule(rule *BracesOnWhenStatementsRule) *api.Rule {
	return &api.Rule{
		ID:         rule.RuleName,
		Category:   rule.RuleSetName,
		Sev:        api.Severity(rule.Sev),
		NodeTypes:  []string{"when_expression"},
		Confidence: rule.Confidence(),
		Check: func(ctx *api.Context) {
			rule.check(ctx)
		},
	}
}

// runBracesIfRule runs BracesOnIfStatements with given config on inline code.
func runBracesIfRule(t *testing.T, singleLine, multiLine, code string) scanner.FindingColumns {
	t.Helper()
	rule := &BracesOnIfStatementsRule{
		BaseRule:   BaseRule{RuleName: "BracesOnIfStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects if/else statements that are missing braces around their bodies."},
		SingleLine: singleLine,
		MultiLine:  multiLine,
	}

	file := parseBracesInline(t, code)
	d := NewDispatcher([]*api.Rule{makeBracesIfV2Rule(rule)}, nil)
	return d.Run(file)
}

// runBracesWhenRule runs BracesOnWhenStatements with given config on inline code.
func runBracesWhenRule(t *testing.T, singleLine, multiLine, code string) scanner.FindingColumns {
	t.Helper()
	rule := &BracesOnWhenStatementsRule{
		BaseRule:   BaseRule{RuleName: "BracesOnWhenStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects when branches that are missing braces around their bodies."},
		SingleLine: singleLine,
		MultiLine:  multiLine,
	}

	file := parseBracesInline(t, code)
	d := NewDispatcher([]*api.Rule{makeBracesWhenV2Rule(rule)}, nil)
	return d.Run(file)
}

// --- BracesOnIfStatements consistent mode ---

func TestBracesOnIfStatements_Consistent_MixedFlags(t *testing.T) {
	// One branch has braces, the other doesn't -> should flag
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Boolean) {
    if (x) foo() else { bar() }
}`)
	if findings.Len() == 0 {
		t.Error("expected findings for mixed braces in if/else chain, got 0")
	}
	for i := 0; i < findings.Len(); i++ {
		if !strings.Contains(findings.MessageAt(i), "consistent") {
			t.Errorf("expected consistent message, got: %s", findings.MessageAt(i))
		}
	}
}

func TestBracesOnIfStatements_Consistent_AllWithoutBraces(t *testing.T) {
	// Both branches have no braces -> consistent, no finding
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Boolean) {
    if (x) foo() else bar()
}`)
	if findings.Len() != 0 {
		t.Errorf("expected no findings when all branches lack braces, got %d", findings.Len())
	}
}

func TestBracesOnIfStatements_Consistent_AllWithBraces(t *testing.T) {
	// Both branches have braces -> consistent, no finding
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Boolean) {
    if (x) { foo() } else { bar() }
}`)
	if findings.Len() != 0 {
		t.Errorf("expected no findings when all branches have braces, got %d", findings.Len())
	}
}

func TestBracesOnIfStatements_Consistent_ElseIfChain(t *testing.T) {
	// Else-if chain with mixed braces
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Int) {
    if (x > 0) { foo() } else if (x < 0) bar() else { baz() }
}`)
	if findings.Len() == 0 {
		t.Error("expected findings for mixed braces in else-if chain, got 0")
	}
}

func TestBracesOnIfStatements_Consistent_ElseIfChain_InnerOnlyBraced(t *testing.T) {
	// Regression: outer then/else are unbraced, but a middle else-if branch
	// is braced. The chain walk must descend into the else-if to see the
	// inconsistency. Before the chain fix, the outer walk treated the
	// else-if wrapper as a single unbraced branch and reported "all
	// consistent" — a false negative.
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Int) {
    if (x > 0) foo() else if (x < 0) { bar() } else baz()
}`)
	if findings.Len() == 0 {
		t.Error("expected findings: middle else-if branch is braced while outer branches are not")
	}
}

func TestBracesOnIfStatements_Consistent_ElseIfChain_AllBraced(t *testing.T) {
	// Regression: every branch in the else-if chain is braced. Before the
	// chain fix, the outer walk saw the else-if control_structure_body
	// wrapper as unbraced (its first child is the nested if_expression, not
	// a `{`), so it falsely reported a mix.
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Int) {
    if (x > 0) { foo() } else if (x < 0) { bar() } else { baz() }
}`)
	if findings.Len() != 0 {
		t.Errorf("expected no findings when every branch in chain is braced, got %d", findings.Len())
	}
}

func TestBracesOnIfStatements_Consistent_MultiLine_MixedFlags(t *testing.T) {
	// Multi-line: one branch has braces, other doesn't
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Boolean) {
    if (x) {
        foo()
    } else
        bar()
}`)
	if findings.Len() == 0 {
		t.Error("expected findings for mixed multi-line braces, got 0")
	}
}

func TestBracesOnIfStatements_Consistent_MultiLine_AllWithout(t *testing.T) {
	// Multi-line: all branches lack braces -> consistent
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Boolean) {
    if (x)
        foo()
    else
        bar()
}`)
	if findings.Len() != 0 {
		t.Errorf("expected no findings when all multi-line branches lack braces, got %d", findings.Len())
	}
}

// --- BracesOnWhenStatements consistent mode ---

func TestBracesOnWhenStatements_Consistent_MixedFlags(t *testing.T) {
	// Some entries have braces, some don't -> should flag
	findings := runBracesWhenRule(t, "consistent", "consistent", `
package test
fun example(x: Int) {
    when (x) {
        1 -> { "one" }
        2 -> "two"
        else -> { "other" }
    }
}`)
	if findings.Len() == 0 {
		t.Error("expected findings for mixed braces in when, got 0")
	}
	for i := 0; i < findings.Len(); i++ {
		if !strings.Contains(findings.MessageAt(i), "consistent") {
			t.Errorf("expected consistent message, got: %s", findings.MessageAt(i))
		}
	}
}

func TestBracesOnWhenStatements_Consistent_AllWithoutBraces(t *testing.T) {
	// All entries without braces -> consistent, no finding
	findings := runBracesWhenRule(t, "consistent", "consistent", `
package test
fun example(x: Int) {
    when (x) {
        1 -> "one"
        2 -> "two"
        else -> "other"
    }
}`)
	if findings.Len() != 0 {
		t.Errorf("expected no findings when all when entries lack braces, got %d", findings.Len())
	}
}

func TestBracesOnWhenStatements_Consistent_AllWithBraces(t *testing.T) {
	// All entries with braces -> consistent, no finding
	findings := runBracesWhenRule(t, "consistent", "consistent", `
package test
fun example(x: Int) {
    when (x) {
        1 -> { "one" }
        2 -> { "two" }
        else -> { "other" }
    }
}`)
	if findings.Len() != 0 {
		t.Errorf("expected no findings when all when entries have braces, got %d", findings.Len())
	}
}

func TestBracesOnWhenStatements_IgnoresTestSources(t *testing.T) {
	rule := &BracesOnWhenStatementsRule{
		BaseRule:   BaseRule{RuleName: "BracesOnWhenStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects when branches that are missing braces around their bodies."},
		SingleLine: "always",
		MultiLine:  "always",
	}
	file := parseBracesPath(t, "src/test/kotlin/FooTest.kt", `
package test
fun example(x: Int) {
    when (x) {
        1 -> "one"
        else -> "other"
    }
}
`)
	d := NewDispatcher([]*api.Rule{makeBracesWhenV2Rule(rule)}, nil)
	findings := d.Run(file)
	if findings.Len() != 0 {
		t.Fatalf("expected no findings for test sources, got %d", findings.Len())
	}
}

func applyFixes(src string, cols scanner.FindingColumns) string {
	type edit struct {
		start, end int
		repl       string
	}
	var edits []edit
	for i := 0; i < cols.Len(); i++ {
		fx := cols.FixAt(i)
		if fx == nil || !fx.ByteMode {
			continue
		}
		edits = append(edits, edit{fx.StartByte, fx.EndByte, fx.Replacement})
	}
	// Descending order so earlier edits don't shift later byte offsets.
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := []byte(src)
	for _, e := range edits {
		out = append(out[:e.start], append([]byte(e.repl), out[e.end:]...)...)
	}
	return string(out)
}

// TestBracesOnIfStatements_ExpressionPosition_ReportsEachBranchWithoutFix follows detekt: an
// expression-position if is checked like a statement, one finding per branch, but with no autofix.
func TestBracesOnIfStatements_ExpressionPosition_ReportsEachBranchWithoutFix(t *testing.T) {
	src := `
package test
fun example(x: Boolean): Int {
    val r = if (x) 1 else 2
    return r
}`
	findings := runBracesIfRule(t, "always", "always", src)
	if findings.Len() != 2 {
		t.Fatalf("expected one finding per branch, got %d", findings.Len())
	}
	if out := applyFixes(src, findings); out != src {
		t.Errorf("expression-position ifs must not be rewritten, got:\n%s", out)
	}
	if got := findings.MessageAt(0); got != "Missing braces on this branch, add them." {
		t.Errorf("unexpected message %q", got)
	}
}

// TestBuildBraceWrapFix_PreservesCommentBetweenHeaderAndBody verifies that a
// comment sitting between `)` and the body is not swallowed by the
// replacement range. Previously the helper extended back to the prev
// sibling's end byte, which deleted any intervening comment.
func TestBuildBraceWrapFix_PreservesCommentBetweenHeaderAndBody(t *testing.T) {
	src := `package test
fun example(x: Boolean) {
    if (x) /* keep me */
        println("ok")
}
`
	findings := runBracesIfRule(t, "always", "always", src)
	if findings.Len() == 0 {
		t.Fatal("expected finding for unbraced if")
	}
	out := applyFixes(src, findings)
	if !strings.Contains(out, "/* keep me */") {
		t.Errorf("comment between header and body was lost. Output:\n%s", out)
	}
}

// TestBuildBraceWrapFix_TabIndentedFile verifies the body indent uses a tab
// step when the surrounding file is tab-indented, instead of hardcoding
// four spaces (which would mix tabs and spaces).
func TestBuildBraceWrapFix_TabIndentedFile(t *testing.T) {
	src := "package test\nfun example(x: Boolean) {\n\tif (x) println(\"ok\")\n}\n"
	findings := runBracesIfRule(t, "always", "always", src)
	if findings.Len() == 0 {
		t.Fatal("expected finding for unbraced if")
	}
	out := applyFixes(src, findings)
	// Body line should start with two tabs (one for the surrounding fun, one
	// for the new brace nesting), not a tab followed by spaces.
	if !strings.Contains(out, "\n\t\tprintln(") {
		t.Errorf("expected tab-stepped body indent in tab-indented file. Output:\n%q", out)
	}
}

func TestBracesOnIfStatements_IgnoresTestSources(t *testing.T) {
	rule := &BracesOnIfStatementsRule{
		BaseRule:   BaseRule{RuleName: "BracesOnIfStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects if/else statements that are missing braces around their bodies."},
		SingleLine: "always",
		MultiLine:  "always",
	}
	file := parseBracesPath(t, "src/test/kotlin/FooTest.kt", `
package test
fun example(x: Boolean) {
    if (x) println("yes")
}
`)
	d := NewDispatcher([]*api.Rule{makeBracesIfV2Rule(rule)}, nil)
	findings := d.Run(file)
	if findings.Len() != 0 {
		t.Fatalf("expected no findings for test sources, got %d", findings.Len())
	}
}

// TestBuildBraceWrapFix_LineCommentBeforeWhenBody verifies the opening brace is
// not appended to a line comment that sits between `->` and the body.
func TestBuildBraceWrapFix_LineCommentBeforeWhenBody(t *testing.T) {
	src := `package test
fun example(x: Int) {
    when {
        x > 1 ->
            // explain
            // more
            println("a")
    }
}
`
	findings := runBracesWhenRule(t, "always", "always", src)
	if findings.Len() == 0 {
		t.Fatal("expected finding for unbraced when branch")
	}
	out := applyFixes(src, findings)
	want := "x > 1 -> {\n            // explain\n            // more\n            println(\"a\")\n        }"
	if !strings.Contains(out, want) {
		t.Errorf("brace must follow the arrow, not a comment. Output:\n%s", out)
	}
}

func TestBracesOnIfStatements_NeverFlagsRedundantBracesNotMissingOnes(t *testing.T) {
	missing := runBracesIfRule(t, "never", "always", `
package test
fun example(x: Boolean) {
    if (x) foo()
}`)
	if missing.Len() != 0 {
		t.Errorf("singleLine never must not ask for braces, got %d findings", missing.Len())
	}
	redundant := runBracesIfRule(t, "never", "always", `
package test
fun example(x: Boolean) {
    if (x) { foo() }
}`)
	if redundant.Len() != 1 || !strings.Contains(redundant.MessageAt(0), "Extra braces exist") {
		t.Errorf("expected one redundant-braces finding, got %d", redundant.Len())
	}
}

func TestBracesOnWhenStatements_NecessaryKeepsMultiStatementBraces(t *testing.T) {
	findings := runBracesWhenRule(t, "necessary", "necessary", `
package test
fun example(x: Int) {
    when (x) {
        1 -> { foo() }
        2 -> {
            bar()
            bar()
        }
        else -> baz()
    }
}`)
	if findings.Len() != 1 {
		t.Errorf("expected only the single-statement braced branch, got %d findings", findings.Len())
	}
}

// TestBuildBraceWrapFix_MultiConditionWhenEntryIndentsBody verifies the wrapped body
// nests one step inside the branch when the arrow shares a line with the last condition.
func TestBuildBraceWrapFix_MultiConditionWhenEntryIndentsBody(t *testing.T) {
	src := `package test
fun example(x: Int) {
    when (x) {
        1,
        2 -> foo()
        else -> bar()
    }
}
`
	findings := runBracesWhenRule(t, "always", "always", src)
	if findings.Len() == 0 {
		t.Fatal("expected finding for unbraced when branch")
	}
	out := applyFixes(src, findings)
	want := "        2 -> {\n            foo()\n        }\n"
	if !strings.Contains(out, want) {
		t.Errorf("expected body indented one step, got:\n%s", out)
	}
}

func TestBracesOnIfStatements_NeverFlagsBracesNotMissingBraces(t *testing.T) {
	findings := runBracesIfRule(t, "never", "always", `
package test
fun example(x: Int) {
    if (x == 1) println(1)
    if (x == 2) { println(2) }
    if (x == 3) { println(3); println(4) }
    if (x == 4) { // keep
        println(5)
    }
    if (x == 5) println(1) else { println(2) }
}`)
	if findings.Len() != 3 {
		t.Fatalf("expected 3 findings (the two single-line braced ifs and the braced else), got %d", findings.Len())
	}
	for i := 0; i < findings.Len(); i++ {
		if !strings.Contains(findings.MessageAt(i), "Extra braces exist") {
			t.Errorf("unexpected message %q", findings.MessageAt(i))
		}
	}
}

// TestBracesOnIfStatements_NeverKeepsDanglingElseBraces reports the braces like detekt but offers no
// fix, because removing them would bind the else to the inner if.
func TestBracesOnIfStatements_NeverKeepsDanglingElseBraces(t *testing.T) {
	src := `
package test
fun example(x: Int, y: Int) {
    if (x == 1) { if (y == 1) println(1) } else println(2)
}`
	findings := runBracesIfRule(t, "never", "never", src)
	if findings.Len() != 1 {
		t.Fatalf("expected the braces to be reported, got %d findings", findings.Len())
	}
	if out := applyFixes(src, findings); out != src {
		t.Errorf("braces protect the else binding, got:\n%s", out)
	}
}

func TestBracesOnIfStatements_AlwaysChecksElseBranch(t *testing.T) {
	findings := runBracesIfRule(t, "always", "always", `
package test
fun example(x: Int) {
    if (x == 1) { println(1) } else println(2)
}`)
	if findings.Len() != 1 {
		t.Errorf("expected else branch without braces to be flagged, got %d", findings.Len())
	}
}

func TestBracesOnWhenStatements_NeverFlagsUnnecessaryBraces(t *testing.T) {
	findings := runBracesWhenRule(t, "never", "always", `
package test
fun example(x: Int) {
    when (x) {
        1 -> { println() }
        2 -> println()
        3 -> { println(); println() }
    }
}`)
	if findings.Len() != 2 {
		t.Errorf("expected both braced branches, got %d", findings.Len())
	}
}
