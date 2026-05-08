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
  - id: CustomInternalEndpointPattern
    ruleset: security
    description: Detects hardcoded internal endpoint URLs in source.
    severity: warning
    nodeTypes: [string_literal, line_string_literal, multi_line_string_literal]
    languages: [kotlin, java]
    match: stringLiteralBody
    pattern: '^https://internal[.]example[.]com/api/v[0-9]+/[a-zA-Z0-9_-]{8,}$'
    message: Hardcoded internal endpoint URL. Load it from configuration or a secret store.
    ignorePlaceholders: true
`))
	t.Cleanup(func() { rules.ApplyConfig(config.NewConfig()) })

	findings := runRuleByName(t, "CustomInternalEndpointPattern", `
const val INTERNAL_API = "https://internal.example.com/api/v2/admin-purge-token"
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "Hardcoded internal endpoint URL") {
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
    pattern: '^https://internal[.]example[.]com/api/.+$'
    message: Hardcoded internal endpoint URL.
    ignorePlaceholders: true
`))
	t.Cleanup(func() { rules.ApplyConfig(config.NewConfig()) })

	findings := runRuleByName(t, "CustomPlaceholderPattern", `
val u = "https://internal.example.com/api/v2/<token>"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}
