package rules

import (
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func parseStyleExpressionsInline(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func firstStyleExpressionsFunction(t *testing.T, file *scanner.File) uint32 {
	t.Helper()
	var fn uint32
	file.FlatWalkNodes(0, "function_declaration", func(idx uint32) {
		if fn == 0 {
			fn = idx
		}
	})
	if fn == 0 {
		t.Fatal("expected function_declaration")
	}
	return fn
}

func runStyleExpressionsRuleByName(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	file := parseStyleExpressionsInline(t, code)
	for _, rule := range v2.Registry {
		if rule.ID == ruleName {
			cols := NewDispatcherV2([]*v2.Rule{rule}).Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func TestCountJumpExpressionsFlatSkipsNestedCallableBodies(t *testing.T) {
	file := parseStyleExpressionsInline(t, `
package test

fun outer(items: List<Int>): Int {
    items.forEach {
        if (it < 0) return@forEach
        if (it == 0) throw IllegalStateException("zero")
    }
    val callback = fun(value: Int): Int {
        if (value < 0) return -1
        if (value == 0) throw IllegalArgumentException("zero")
        return value
    }
    if (items.isEmpty()) throw IllegalStateException("empty")
    if (items.size == 1) return 1
    return callback(items.first())
}
`)
	fn := firstStyleExpressionsFunction(t, file)

	if got := countJumpExpressionsFlat(fn, file, "return", 100, nil); got != 2 {
		t.Fatalf("expected only outer returns to be counted, got %d", got)
	}
	if got := countJumpExpressionsFlat(fn, file, "throw", 100, nil); got != 1 {
		t.Fatalf("expected only outer throws to be counted, got %d", got)
	}
}

func TestJumpMetricsFlatSkipsAnonymousFunctions(t *testing.T) {
	file := parseStyleExpressionsInline(t, `
package test

fun outer(items: List<Int>) {
    if (items.isEmpty()) throw IllegalStateException("empty")
    if (items.size == 1) throw IllegalArgumentException("single")
    val callback = fun(value: Int) {
        if (value < 0) throw IllegalStateException("negative")
    }
}
`)
	fn := firstStyleExpressionsFunction(t, file)

	if got := getJumpMetricsFlat(fn, file).throws; got != 2 {
		t.Fatalf("expected only outer throws to be counted, got %d", got)
	}
}

func TestThrowsCountIgnoresNestedCallableThrows(t *testing.T) {
	findings := runStyleExpressionsRuleByName(t, "ThrowsCount", `
package test

fun validate(items: List<Int>) {
    if (items.isEmpty()) throw IllegalStateException("empty")
    if (items.size == 1) throw IllegalArgumentException("single")
    items.forEach {
        if (it < 0) throw IllegalStateException("negative")
    }
    val callback = fun(value: Int) {
        if (value == 0) throw IllegalArgumentException("zero")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when only nested callable throws exceed the limit, got %d", len(findings))
	}
}

func TestReturnCountIgnoresNestedCallableReturns(t *testing.T) {
	findings := runStyleExpressionsRuleByName(t, "ReturnCount", `
package test

fun classify(items: List<Int>): Int {
    items.forEach {
        if (it < 0) return@forEach
        if (it == 0) return@forEach
    }
    val callback = fun(value: Int): Int {
        if (value < 0) return -1
        if (value == 0) return 0
        return value
    }
    if (items.isEmpty()) return 0
    return callback(items.first())
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when nested callable returns would be the only excess returns, got %d", len(findings))
	}
}
