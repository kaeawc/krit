package rules_test

import "testing"

func TestSuppressedWarningWithoutJustification_PropertyPositive(t *testing.T) {
	findings := runRuleByName(t, "SuppressedWarningWithoutJustification", `
package test

@Suppress("UNCHECKED_CAST")
val m: Map<String, Int> = mapOf<Any, Any>() as Map<String, Int>
`)
	count := 0
	for _, f := range findings {
		if f.Rule == "SuppressedWarningWithoutJustification" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 finding, got %d", count)
	}
}

func TestSuppressedWarningWithoutJustification_KdocNegative(t *testing.T) {
	findings := runRuleByName(t, "SuppressedWarningWithoutJustification", `
package test

/** Safe: factory always produces String->Int. */
@Suppress("UNCHECKED_CAST")
val m: Map<String, Int> = mapOf<Any, Any>() as Map<String, Int>
`)
	for _, f := range findings {
		if f.Rule == "SuppressedWarningWithoutJustification" {
			t.Fatalf("did not expect finding: %s", f.Message)
		}
	}
}

func TestSuppressedWarningWithoutJustification_NonSuppressAnnotationNegative(t *testing.T) {
	findings := runRuleByName(t, "SuppressedWarningWithoutJustification", `
package test

@Deprecated("old")
fun stale() {}
`)
	for _, f := range findings {
		if f.Rule == "SuppressedWarningWithoutJustification" {
			t.Fatalf("did not expect finding: %s", f.Message)
		}
	}
}
