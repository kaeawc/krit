package rules_test

import "testing"

func TestUpperLowerInvariantMisuse_Positive(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
fun main(userName: String, view: TextView) {
    view.text = userName.uppercase()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for uppercase() assigned to UI text, got %d", len(findings))
	}
}

func TestUpperLowerInvariantMisuse_NegativeNonUIUse(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
fun main(userName: String, email: String) {
    val n = userName.uppercase()
    val same = email.lowercase() == "a@b.c"
    store(email.lowercase())
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for locale-invariant uses, got %d", len(findings))
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
fun main(label: String, view: TextView) {
    view.setText(label.lowercase())
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for bare lowercase(), got %d", len(findings))
	}
}

func TestUpperLowerInvariantMisuse_NegativeInvariantTokens(t *testing.T) {
	tests := map[string]string{
		"enum name":       `fun f(mode: Mode) = mode.name.lowercase()`,
		"qualified name":  `fun f() = Mode.FAST.name.uppercase()`,
		"string literal":  `fun f() = "Token".lowercase()`,
		"valueOf lookup":  `fun f(raw: String) = Mode.valueOf(raw.uppercase())`,
		"enumValueOf":     `fun f(raw: String) = enumValueOf<Mode>(raw.uppercase())`,
		"enum this label": `enum class Level { LOW; val label get() = toString().lowercase() }`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "UpperLowerInvariantMisuse", "package test\nenum class Mode { FAST }\n"+body+"\n")
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
			}
		})
	}
}

func TestUpperLowerInvariantMisuse_NegativeEnumToStringOnEnumTypedParameter(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
enum class Mode { FAST }
fun f(mode: Mode) = mode.toString().lowercase()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for enum toString(), got %d", len(findings))
	}
}

func TestUpperLowerInvariantMisuse_PositiveUserText(t *testing.T) {
	findings := runRuleByName(t, "UpperLowerInvariantMisuse", `
package test
fun f(title: String, input: Any, binding: Binding) {
    Text(text = "Hi ${title.uppercase()}")
    binding.heading.text = input.toString().lowercase()
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for user-facing text, got %d", len(findings))
	}
}
