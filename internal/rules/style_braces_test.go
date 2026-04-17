package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// runBracesIfRule runs BracesOnIfStatements with given config on inline code.
func runBracesIfRule(t *testing.T, singleLine, multiLine, code string) []scanner.Finding {
	t.Helper()
	rule := &rules.BracesOnIfStatementsRule{
		BaseRule:    rules.BaseRule{RuleName: "BracesOnIfStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects if/else statements that are missing braces around their bodies."},
		SingleLine:  singleLine,
		MultiLine:   multiLine,
	}

	file := parseInline(t, code)
	d := rules.NewDispatcher([]rules.Rule{rule})
	return d.Run(file)
}

// runBracesWhenRule runs BracesOnWhenStatements with given config on inline code.
func runBracesWhenRule(t *testing.T, singleLine, multiLine, code string) []scanner.Finding {
	t.Helper()
	rule := &rules.BracesOnWhenStatementsRule{
		BaseRule:    rules.BaseRule{RuleName: "BracesOnWhenStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects when branches that are missing braces around their bodies."},
		SingleLine:  singleLine,
		MultiLine:   multiLine,
	}

	file := parseInline(t, code)
	d := rules.NewDispatcher([]rules.Rule{rule})
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
	if len(findings) == 0 {
		t.Error("expected findings for mixed braces in if/else chain, got 0")
	}
	for _, f := range findings {
		if !strings.Contains(f.Message, "consistent") {
			t.Errorf("expected consistent message, got: %s", f.Message)
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
	if len(findings) != 0 {
		t.Errorf("expected no findings when all branches lack braces, got %d", len(findings))
	}
}

func TestBracesOnIfStatements_Consistent_AllWithBraces(t *testing.T) {
	// Both branches have braces -> consistent, no finding
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Boolean) {
    if (x) { foo() } else { bar() }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings when all branches have braces, got %d", len(findings))
	}
}

func TestBracesOnIfStatements_Consistent_ElseIfChain(t *testing.T) {
	// Else-if chain with mixed braces
	findings := runBracesIfRule(t, "consistent", "consistent", `
package test
fun example(x: Int) {
    if (x > 0) { foo() } else if (x < 0) bar() else { baz() }
}`)
	if len(findings) == 0 {
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
	if len(findings) == 0 {
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
	if len(findings) != 0 {
		t.Errorf("expected no findings when all multi-line branches lack braces, got %d", len(findings))
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
	if len(findings) == 0 {
		t.Error("expected findings for mixed braces in when, got 0")
	}
	for _, f := range findings {
		if !strings.Contains(f.Message, "consistent") {
			t.Errorf("expected consistent message, got: %s", f.Message)
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
	if len(findings) != 0 {
		t.Errorf("expected no findings when all when entries lack braces, got %d", len(findings))
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
	if len(findings) != 0 {
		t.Errorf("expected no findings when all when entries have braces, got %d", len(findings))
	}
}
