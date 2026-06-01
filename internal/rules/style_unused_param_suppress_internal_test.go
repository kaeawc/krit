package rules

import (
	"testing"
)

// TestParameterHasUnusedSuppression_UpperCaseDiagnostic pins the regression: a
// `@Suppress("UNUSED_PARAMETER")` annotation parses as a `parameter_modifiers`
// sibling of the `parameter` node, so the suppression must be detected even
// though the annotation is not part of the parameter node's own text.
func TestParameterHasUnusedSuppression_UpperCaseDiagnostic(t *testing.T) {
	code := "package test\n" +
		"class R {\n" +
		"    private fun s(@Suppress(\"UNUSED_PARAMETER\") targetInfo: TargetInfo): Int = 0\n" +
		"}\n"
	f := parseInlineForInternalTest(t, code)
	var param uint32
	f.FlatWalkNodes(0, "parameter", func(p uint32) {
		if param == 0 && extractIdentifierFlat(f, p) == "targetInfo" {
			param = p
		}
	})
	if param == 0 {
		t.Fatal("expected to find the targetInfo parameter")
	}
	if !parameterHasUnusedSuppression(f, param) {
		t.Fatal("@Suppress(\"UNUSED_PARAMETER\") on a parameter must be honored")
	}
}

// TestParameterHasUnusedSuppression_LowerCaseDiagnostic covers the alternate
// `@Suppress("unused")` spelling.
func TestParameterHasUnusedSuppression_LowerCaseDiagnostic(t *testing.T) {
	code := "package test\n" +
		"class R {\n" +
		"    private fun s(@Suppress(\"unused\") leftover: TargetInfo): Int = 0\n" +
		"}\n"
	f := parseInlineForInternalTest(t, code)
	var param uint32
	f.FlatWalkNodes(0, "parameter", func(p uint32) {
		if param == 0 && extractIdentifierFlat(f, p) == "leftover" {
			param = p
		}
	})
	if param == 0 {
		t.Fatal("expected to find the leftover parameter")
	}
	if !parameterHasUnusedSuppression(f, param) {
		t.Fatal("@Suppress(\"unused\") on a parameter must be honored")
	}
}

// TestParameterHasUnusedSuppression_Unannotated is the negative guard: a plain
// parameter (no annotation, or an unrelated @Suppress) must not be treated as
// suppressed, so genuinely-unused parameters are still reported.
func TestParameterHasUnusedSuppression_Unannotated(t *testing.T) {
	code := "package test\n" +
		"class R {\n" +
		"    private fun s(plain: TargetInfo, @Suppress(\"NOTHING\") other: TargetInfo): Int = 0\n" +
		"}\n"
	f := parseInlineForInternalTest(t, code)
	for _, name := range []string{"plain", "other"} {
		var param uint32
		f.FlatWalkNodes(0, "parameter", func(p uint32) {
			if param == 0 && extractIdentifierFlat(f, p) == name {
				param = p
			}
		})
		if param == 0 {
			t.Fatalf("expected to find the %s parameter", name)
		}
		if parameterHasUnusedSuppression(f, param) {
			t.Fatalf("parameter %s must not be treated as unused-suppressed", name)
		}
	}
}
