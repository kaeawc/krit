package rules_test

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// These tests exercise the inline-snippet test helpers via
// api.FakeRule, so a regression in the helper wiring (parse →
// dispatch → return findings) is caught independently of any real
// rule's behavior.

const inlineHelperKotlin = `package test

fun greet() {
    println("hi")
}
`

func TestInlineHelpers_FindingsForReturnsFakeRuleEmissions(t *testing.T) {
	file := inlineKotlin(t, inlineHelperKotlin)

	emits := api.FakeRule("InlineHelpersFakeEmits",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(1, 1, "fake finding")
		}),
	)

	findings := findingsFor(t, emits, file)
	if len(findings) == 0 {
		t.Fatalf("expected findings from emitting fake rule, got none")
	}
	for _, f := range findings {
		if f.Rule != "InlineHelpersFakeEmits" {
			t.Errorf("unexpected rule on finding: %s", f.Rule)
		}
	}
}

func TestInlineHelpers_FindingsForReturnsEmptyForSilentRule(t *testing.T) {
	file := inlineKotlin(t, inlineHelperKotlin)

	silent := api.FakeRule("InlineHelpersFakeSilent",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {}),
	)

	findings := findingsFor(t, silent, file)
	if len(findings) != 0 {
		t.Fatalf("expected zero findings from silent fake rule, got %d", len(findings))
	}
}

func TestInlineHelpers_AssertFlagsOnPassesWhenRuleEmits(t *testing.T) {
	file := inlineKotlin(t, inlineHelperKotlin)

	emits := api.FakeRule("InlineHelpersAssertFlagsPos",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(1, 1, "fake finding")
		}),
	)

	got := assertFlagsOn(t, emits, file)
	if len(got) == 0 {
		t.Fatalf("assertFlagsOn returned empty findings slice when expecting >=1")
	}
}

func TestInlineHelpers_AssertCleanOnPassesWhenRuleSilent(t *testing.T) {
	file := inlineKotlin(t, inlineHelperKotlin)

	silent := api.FakeRule("InlineHelpersAssertCleanPos",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {}),
	)

	// If this calls t.Fatalf, the test fails — which is the assertion
	// we want.
	assertCleanOn(t, silent, file)
}

func TestInlineHelpers_FlagsAssertionFailsForEmptyFindings(t *testing.T) {
	rule := api.FakeRule("InlineHelpersFlagsPredicate")
	file := inlineKotlin(t, inlineHelperKotlin)

	ok, msg := flagsAssertion(rule, file, nil)
	if ok {
		t.Fatalf("flagsAssertion should fail for empty findings")
	}
	if msg == "" {
		t.Errorf("expected non-empty failure message")
	}
	if ok2, _ := flagsAssertion(rule, file, []scanner.Finding{{Line: 1}}); !ok2 {
		t.Errorf("flagsAssertion should pass with at least one finding")
	}
}

func TestInlineHelpers_CleanAssertionFailsForNonEmptyFindings(t *testing.T) {
	rule := api.FakeRule("InlineHelpersCleanPredicate")
	file := inlineKotlin(t, inlineHelperKotlin)

	ok, msg := cleanAssertion(rule, file, []scanner.Finding{{Line: 3, Message: "boom"}})
	if ok {
		t.Fatalf("cleanAssertion should fail for non-empty findings")
	}
	if msg == "" {
		t.Errorf("expected non-empty failure message")
	}
	if ok2, _ := cleanAssertion(rule, file, nil); !ok2 {
		t.Errorf("cleanAssertion should pass with no findings")
	}
}

func TestInlineHelpers_FindRuleByIDFindsRegisteredRule(t *testing.T) {
	if len(api.Registry) == 0 {
		t.Skip("registry empty in this build configuration")
	}
	first := api.Registry[0]
	if first == nil || first.ID == "" {
		t.Fatalf("first registry entry is missing an ID: %+v", first)
	}
	got := findRuleByID(t, first.ID)
	if got != first {
		t.Errorf("findRuleByID returned %p, want %p", got, first)
	}
}
