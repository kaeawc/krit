package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func parseStyleExpressionsInline(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
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
	for _, rule := range api.Registry {
		if rule.ID == ruleName {
			cols := NewDispatcher([]*api.Rule{rule}, nil).Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func TestCountJumpExpressionsFlatSkipsNestedFunctionBodiesButCountsLambdas(t *testing.T) {
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

	if got := countJumpExpressionsFlat(fn, file, "return", 100, nil); got != 3 {
		t.Fatalf("expected the lambda and outer returns but not the anonymous function's, got %d", got)
	}
	if got := countJumpExpressionsFlat(fn, file, "throw", 100, nil); got != 2 {
		t.Fatalf("expected the lambda and outer throws but not the anonymous function's, got %d", got)
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

func TestThrowsCountIgnoresAnonymousFunctionThrowsButCountsLambdaThrows(t *testing.T) {
	findings := runStyleExpressionsRuleByName(t, "ThrowsCount", `
package test

fun validate(items: List<Int>) {
    if (items.isEmpty()) throw IllegalStateException("empty")
    val callback = fun(value: Int) {
        if (value == 0) throw IllegalArgumentException("zero")
        if (value == 1) throw IllegalArgumentException("one")
    }
    items.forEach {
        if (it < 0) throw IllegalStateException("negative")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("anonymous function throws are not the outer function's, got %d", len(findings))
	}
	findings = runStyleExpressionsRuleByName(t, "ThrowsCount", `
package test

fun validate(items: List<Int>) {
    if (items.isEmpty()) throw IllegalStateException("empty")
    if (items.size == 1) throw IllegalArgumentException("single")
    items.forEach {
        if (it < 0) throw IllegalStateException("negative")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("a throw inside an inline lambda belongs to the function, got %d", len(findings))
	}
	want := "Too many throw statements in the function validate. The maximum number of allowed throw statements is 2."
	if findings[0].Message != want {
		t.Fatalf("message = %q, want %q", findings[0].Message, want)
	}
}

func TestThrowsCountSkipsExpressionBodiedFunctions(t *testing.T) {
	findings := runStyleExpressionsRuleByName(t, "ThrowsCount", `
package test

fun pick(kind: Int): String =
    when (kind) {
        0 -> "a"
        1 -> throw IllegalStateException("one")
        2 -> throw IllegalStateException("two")
        else -> throw IllegalArgumentException("other")
    }

fun wrap(block: () -> Unit) = run {
    if (true) throw IllegalStateException("a")
    if (false) throw IllegalStateException("b")
    throw IllegalStateException("c")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expression-bodied functions are not checked by detekt, got %d", len(findings))
	}
	findings = runStyleExpressionsRuleByName(t, "ThrowsCount", `
package test

fun pick(kind: Int): String {
    return when (kind) {
        0 -> "a"
        1 -> throw IllegalStateException("one")
        2 -> throw IllegalStateException("two")
        else -> throw IllegalArgumentException("other")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("block-bodied function with three throws must still be reported, got %d", len(findings))
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
