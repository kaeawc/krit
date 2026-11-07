package rules_test

import "testing"

func TestTextDirectionLiteralInString_RawStringDoesNotProcessEscapes(t *testing.T) {
	findings := runRuleByName(t, "TextDirectionLiteralInString", "package test\n\nval a = \"\"\"\\u200E\"\"\"\nval b = \"\\u200E\"\n")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (regular string only), got %d: %v", len(findings), findings)
	}
}
