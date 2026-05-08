package rules_test

import (
	"strings"
	"testing"
)

func TestRequiresOptInWithoutMessage_Positive(t *testing.T) {
	findings := runRuleByName(t, "RequiresOptInWithoutMessage", `
package test

@RequiresOptIn
annotation class InternalApi
`)
	count := 0
	for _, f := range findings {
		if f.Rule == "RequiresOptInWithoutMessage" {
			count++
			if !strings.Contains(f.Message, "message") {
				t.Errorf("expected message to mention 'message', got %q", f.Message)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 finding, got %d", count)
	}
}

func TestRequiresOptInWithoutMessage_PositiveEmptyParens(t *testing.T) {
	findings := runRuleByName(t, "RequiresOptInWithoutMessage", `
package test

@RequiresOptIn()
annotation class InternalApi
`)
	count := 0
	for _, f := range findings {
		if f.Rule == "RequiresOptInWithoutMessage" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 finding, got %d", count)
	}
}

func TestRequiresOptInWithoutMessage_NegativeWithMessage(t *testing.T) {
	findings := runRuleByName(t, "RequiresOptInWithoutMessage", `
package test

@RequiresOptIn(message = "Internal API — subject to change.")
annotation class InternalApi
`)
	for _, f := range findings {
		if f.Rule == "RequiresOptInWithoutMessage" {
			t.Fatalf("did not expect finding: %s", f.Message)
		}
	}
}

func TestRequiresOptInWithoutMessage_NegativeWithLevelAndMessage(t *testing.T) {
	findings := runRuleByName(t, "RequiresOptInWithoutMessage", `
package test

@RequiresOptIn(level = RequiresOptIn.Level.ERROR, message = "Use with care.")
annotation class InternalApi
`)
	for _, f := range findings {
		if f.Rule == "RequiresOptInWithoutMessage" {
			t.Fatalf("did not expect finding: %s", f.Message)
		}
	}
}

func TestRequiresOptInWithoutMessage_NegativeOtherAnnotation(t *testing.T) {
	findings := runRuleByName(t, "RequiresOptInWithoutMessage", `
package test

@OptIn(SomeMarker::class)
fun usesIt() = Unit
`)
	for _, f := range findings {
		if f.Rule == "RequiresOptInWithoutMessage" {
			t.Fatalf("did not expect finding: %s", f.Message)
		}
	}
}
