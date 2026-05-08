package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
)

func TestCustomPatternRule_StringLiteralBody(t *testing.T) {
	rules.ApplyConfig(loadTempConfig(t, `
customRules:
  - id: CustomWebhookPattern
    ruleset: security
    description: Detects configured webhook URLs in source.
    severity: warning
    nodeTypes: [string_literal, line_string_literal, multi_line_string_literal]
    languages: [kotlin, java]
    match: stringLiteralBody
    pattern: '^https://hooks[.]example[.]com/services/T[A-Z0-9]{9,12}/B[A-Z0-9]{9,12}/[A-Za-z0-9_-]{24,}$'
    message: Configured webhook URL. Load it from configuration or a secret store.
    ignorePlaceholders: true
`))
	t.Cleanup(func() { rules.ApplyConfig(config.NewConfig()) })

	findings := runRuleByName(t, "CustomWebhookPattern", `
const val ALERT_WEBHOOK = "https://hooks.example.com/services/TAAAAAAAAA/BAAAAAAAAA/AAAAAAAAAAAAAAAAAAAAAAAA"
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "Configured webhook URL") {
		t.Fatalf("unexpected message: %q", findings[0].Message)
	}
}

func TestCustomPatternRule_IgnoresPlaceholders(t *testing.T) {
	rules.ApplyConfig(loadTempConfig(t, `
customRules:
  - id: CustomPlaceholderPattern
    ruleset: security
    nodeTypes: [string_literal]
    match: stringLiteralBody
    pattern: '^https://hooks[.]example[.]com/services/.+$'
    message: Configured webhook URL.
    ignorePlaceholders: true
`))
	t.Cleanup(func() { rules.ApplyConfig(config.NewConfig()) })

	findings := runRuleByName(t, "CustomPlaceholderPattern", `
val w = "https://hooks.example.com/services/<team>/<channel>/<secret-token>"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}
