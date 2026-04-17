package rules_test

import "testing"

func TestHardcodedBearerToken_PositiveDirectLiteral(t *testing.T) {
	findings := runRuleByName(t, "HardcodedBearerToken", `
package test

fun send(request: Request) {
    request.header("Authorization", "Bearer sk_live_abcdef0123456789")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestHardcodedBearerToken_PositiveLiteralTemplateExpression(t *testing.T) {
	findings := runRuleByName(t, "HardcodedBearerToken", "package test\n\n"+
		"fun send(request: Request) {\n"+
		"    request.header(\"Authorization\", \"Bearer ${\\\"sk_live_abcdef0123456789\\\"}\")\n"+
		"}\n")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestHardcodedBearerToken_NegativeInterpolatedVariable(t *testing.T) {
	findings := runRuleByName(t, "HardcodedBearerToken", `
package test

object BuildConfig {
    const val API_TOKEN = "sk_live_abcdef0123456789"
}

fun send(request: Request) {
    request.header("Authorization", "Bearer ${BuildConfig.API_TOKEN}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestHardcodedBearerToken_NegativePlaceholder(t *testing.T) {
	findings := runRuleByName(t, "HardcodedBearerToken", `
package test

fun send(request: Request) {
    request.header("Authorization", "Bearer your_api_token_here")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}
