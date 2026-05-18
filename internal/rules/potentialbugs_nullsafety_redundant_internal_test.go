package rules

import (
	"context"
	"os"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// parseInlineForInternalTest parses the given Kotlin source into a *scanner.File
// using a temporary .kt file so internal tests can exercise helpers without
// going through the rules_test wrappers.
func parseInlineForInternalTest(t *testing.T, code string) *scanner.File {
	t.Helper()
	tmp, err := os.CreateTemp("", "krit_nullsafety_*.kt")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })
	if _, err := tmp.WriteString(code); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp: %v", err)
	}
	file, err := scanner.ParseFile(context.Background(), tmp.Name())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return file
}

// TestFlatNavigationSafeCallOperator_LocatesAstToken confirms the helper
// returns the AST node index of the top-level `?.` operator regardless of
// whether the receiver text contains the literal `?.` substring (e.g. in a
// string argument to a chained call). Using AST byte ranges is what keeps
// the autofix from corrupting source containing such literals.
func TestFlatNavigationSafeCallOperator_LocatesAstToken(t *testing.T) {
	file := parseInlineForInternalTest(t, "fun f(): Int { val s: String = \"\"; return s?.length ?: 0 }\n")
	var nav uint32
	file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) {
		if nav == 0 {
			nav = idx
		}
	})
	if nav == 0 {
		t.Fatal("expected to find a navigation_expression")
	}
	op := flatNavigationSafeCallOperator(file, nav)
	if op == 0 {
		t.Fatal("expected flatNavigationSafeCallOperator to find the ?. token")
	}
	if got := file.FlatType(op); got != "?." {
		t.Fatalf("operator type = %q, want %q", got, "?.")
	}
	span := file.FlatEndByte(op) - file.FlatStartByte(op)
	if span != 2 {
		t.Fatalf("operator byte span = %d, want 2", span)
	}
}

// TestFlatNavigationSafeCallOperator_ReceiverStringLiteralWithSafeCall
// covers the Bug A scenario: a receiver call whose string-literal argument
// contains the literal substring `?.`. The AST locator must point at the
// real `?.` operator token, not at the first text occurrence (which lies
// inside the string literal). The old `strings.Index(text, "?.")` approach
// would have produced an offset inside the literal, corrupting the source
// when the autofix replaces the byte range with `.`.
func TestFlatNavigationSafeCallOperator_ReceiverStringLiteralWithSafeCall(t *testing.T) {
	src := "fun f(lookup: (String) -> String): Int { return lookup(\"?.\")?.length ?: 0 }\n"
	file := parseInlineForInternalTest(t, src)
	var nav uint32
	// Pick the outermost navigation_expression (the one immediately holding
	// the top-level ?. operator).
	file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) {
		if nav == 0 {
			nav = idx
		}
	})
	if nav == 0 {
		t.Fatal("expected to find a navigation_expression")
	}
	op := flatNavigationSafeCallOperator(file, nav)
	if op == 0 {
		t.Fatal("expected flatNavigationSafeCallOperator to find the ?. token")
	}
	// Confirm the AST locator landed AFTER the string literal "?.": the
	// text-search alternative would have returned the first `?.` occurrence,
	// which is inside the literal.
	astStart := int(file.FlatStartByte(op))
	textStart := -1
	// Search inside the navigation_expression text for "?." — replicate the
	// old behavior to prove the AST locator differs.
	navText := file.FlatNodeText(nav)
	for i := 0; i+1 < len(navText); i++ {
		if navText[i] == '?' && navText[i+1] == '.' {
			textStart = int(file.FlatStartByte(nav)) + i
			break
		}
	}
	if textStart < 0 {
		t.Fatal("test setup error: expected `?.` substring inside the navigation_expression text")
	}
	if astStart <= textStart {
		t.Fatalf("AST operator start (%d) must lie strictly after the first textual `?.` occurrence (%d); the AST locator should skip past the string literal", astStart, textStart)
	}
	// Quick sanity: the byte range should bracket the real `?.` operator.
	got := navText[astStart-int(file.FlatStartByte(nav)) : astStart-int(file.FlatStartByte(nav))+2]
	if got != "?." {
		t.Fatalf("AST operator bytes = %q, want %q", got, "?.")
	}
}

// TestUnnecessarySafeCallLambdaHasRepeatedThisSafeCalls_IgnoresStringLiteral
// is the Bug B regression: the heuristic must count navigation_expression
// nodes whose receiver is `this_expression` and whose operator is `?.`, not
// raw text occurrences of `this?.`. A string literal containing
// `"this?.foo this?.bar"` and a single real `this?.X` should yield count==1,
// so the helper must return false.
func TestUnnecessarySafeCallLambdaHasRepeatedThisSafeCalls_IgnoresStringLiteral(t *testing.T) {
	file := parseInlineForInternalTest(t, `
package test
fun addKeyValue(k: String, v: String) { println(k + v) }
fun example() {
    val obj: String = "hello"
    with(obj) {
        addKeyValue("k", "this?.foo this?.bar")
        val len = this?.length
        println(len)
    }
}
`)
	var lambda uint32
	file.FlatWalkNodes(0, "lambda_literal", func(idx uint32) {
		if lambda == 0 {
			lambda = idx
		}
	})
	if lambda == 0 {
		t.Fatal("expected to find a lambda_literal")
	}
	if unnecessarySafeCallLambdaHasRepeatedThisSafeCalls(file, lambda) {
		t.Fatal("heuristic must not trip on string-literal occurrences of `this?.`; only real AST navigation should count")
	}
}

// TestUnnecessarySafeCallLambdaHasRepeatedThisSafeCalls_CountsRealCalls
// confirms the AST-based count still trips when the lambda actually has two
// or more `this?.X` navigations.
func TestUnnecessarySafeCallLambdaHasRepeatedThisSafeCalls_CountsRealCalls(t *testing.T) {
	file := parseInlineForInternalTest(t, `
package test
fun example() {
    val obj: String = "hello"
    with(obj) {
        val a = this?.length
        val b = this?.hashCode()
        println(a.toString() + b.toString())
    }
}
`)
	var lambda uint32
	file.FlatWalkNodes(0, "lambda_literal", func(idx uint32) {
		if lambda == 0 {
			lambda = idx
		}
	})
	if lambda == 0 {
		t.Fatal("expected to find a lambda_literal")
	}
	if !unnecessarySafeCallLambdaHasRepeatedThisSafeCalls(file, lambda) {
		t.Fatal("heuristic should trip when the lambda has two real this?.X navigations")
	}
}

// TestGetterNullableReceiverFlat_BacktickColonInPropertyName_Internal pins
// Bug C: the receiver-type-prefix fallback must anchor to the
// variable_declaration child of the property_declaration, not the first `:`
// in the text (which can land inside a backtick-quoted property name).
//
// A backtick-quoted property name containing both `?.` and `:` (where the
// `:` appears AFTER the `?.`) must not be misread as an extension property
// with a nullable receiver. With the old `strings.Index(text, ":")`
// fallback, the `:` inside backticks shifts the boundary so that the
// apparent "receiver-type prefix" includes the `?.` from the name,
// falsely classifying the property as having a nullable receiver.
func TestGetterNullableReceiverFlat_BacktickColonInPropertyName_Internal(t *testing.T) {
	file := parseInlineForInternalTest(t, "val `?.weird:name`: String\n  get() = \"\"\n")
	var getter uint32
	file.FlatWalkNodes(0, "getter", func(idx uint32) {
		if getter == 0 {
			getter = idx
		}
	})
	if getter == 0 {
		t.Fatal("expected to find a getter")
	}
	if getterNullableReceiverFlat(file, getter, false) {
		t.Fatal("getterNullableReceiverFlat must not return true when `?.` appears only inside a backtick-quoted property name")
	}
	if getterNullableReceiverFlat(file, getter, true) {
		t.Fatal("getterNullableReceiverFlat (structural) must not return true when `?.` appears only inside a backtick-quoted property name")
	}
}
