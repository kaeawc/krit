package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
	f, err := scanner.ParseFile(path)
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
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// makeBracesIfV2Rule wraps a BracesOnIfStatementsRule in a native v2.Rule,
// delegating to the (unexported) checkFlatNode implementation.
func makeBracesIfV2Rule(rule *BracesOnIfStatementsRule) *v2.Rule {
	return &v2.Rule{
		ID:         rule.RuleName,
		Category:   rule.RuleSetName,
		Sev:        v2.Severity(rule.Sev),
		NodeTypes:  []string{"if_expression"},
		Confidence: rule.Confidence(),
		Check: func(ctx *v2.Context) {
			rule.check(ctx)
		},
	}
}

// makeBracesWhenV2Rule wraps a BracesOnWhenStatementsRule in a native v2.Rule.
func makeBracesWhenV2Rule(rule *BracesOnWhenStatementsRule) *v2.Rule {
	return &v2.Rule{
		ID:         rule.RuleName,
		Category:   rule.RuleSetName,
		Sev:        v2.Severity(rule.Sev),
		NodeTypes:  []string{"when_entry"},
		Confidence: rule.Confidence(),
		Check: func(ctx *v2.Context) {
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
	d := NewDispatcherV2([]*v2.Rule{makeBracesIfV2Rule(rule)})
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
	d := NewDispatcherV2([]*v2.Rule{makeBracesWhenV2Rule(rule)})
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
fun example(x: Int): String {
    return when (x) {
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
fun example(x: Int): String {
    return when (x) {
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
fun example(x: Int): String {
    return when (x) {
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
fun example(x: Int): String {
    return when (x) {
        1 -> "one"
        else -> "other"
    }
}
`)
	d := NewDispatcherV2([]*v2.Rule{makeBracesWhenV2Rule(rule)})
	findings := d.Run(file)
	if findings.Len() != 0 {
		t.Fatalf("expected no findings for test sources, got %d", findings.Len())
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
	d := NewDispatcherV2([]*v2.Rule{makeBracesIfV2Rule(rule)})
	findings := d.Run(file)
	if findings.Len() != 0 {
		t.Fatalf("expected no findings for test sources, got %d", findings.Len())
	}
}
