package rules_test

import "testing"

func TestUpperLowerInvariantMisuse_Positive(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
fun main(userName: String) {
    val n = userName.uppercase()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for bare uppercase(), got %d", len(findings))
	}
}

func TestUpperLowerInvariantMisuse_NegativeExplicitLocale(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
import java.util.Locale
fun main(userName: String) {
    val n = userName.uppercase(Locale.ROOT)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUpperLowerInvariantMisuse_NegativeAsciiInvariantReceiver(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
fun main(currencyCode: String) {
    val n = currencyCode.uppercase()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for ASCII-invariant receiver, got %d", len(findings))
	}
}

func TestUpperLowerInvariantMisuse_NegativeLocalLookalike(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
fun uppercase(): String = "X"
fun main() {
    val n = uppercase()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for local function with no receiver, got %d", len(findings))
	}
}

func TestUpperLowerInvariantMisuse_LowercasePositive(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
fun main(email: String) {
    val n = email.lowercase()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for bare lowercase(), got %d", len(findings))
	}
}
