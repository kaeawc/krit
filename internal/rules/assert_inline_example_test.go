package rules_test

import "testing"

// Demonstration tests for the inline-snippet rule-test pattern. These
// double as a smoke test that assertFlags / assertClean route a real
// rule end-to-end through the dispatcher and produce the expected
// pass/fail result. New regression tests should follow this shape.

func TestInlineSnippet_EmptyFunctionBlockFlagsBlankBody(t *testing.T) {
	assertFlags(t, "EmptyFunctionBlock", `package test

fun blank() {}
`)
}

func TestInlineSnippet_EmptyFunctionBlockSkipsNonEmptyBody(t *testing.T) {
	assertClean(t, "EmptyFunctionBlock", `package test

fun nonBlank() {
    println("hi")
}
`)
}
