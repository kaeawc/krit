package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func runRuleByNameOnPath(t *testing.T, ruleName, filename, code string) []scanner.Finding {
	t.Helper()

	path := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range rules.Registry {
		if r.Name() == ruleName {
			d := rules.NewDispatcher([]rules.Rule{r})
			return d.Run(file)
		}
	}

	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func TestHardcodedGcpServiceAccount_PositiveServiceAccountJSON(t *testing.T) {
	findings := runRuleByName(t, "HardcodedGcpServiceAccount", `
package test

val serviceAccount = """
    {"type": "service_account", "project_id": "my-proj"}
""".trimIndent()
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestHardcodedGcpServiceAccount_PositivePrivateKey(t *testing.T) {
	findings := runRuleByName(t, "HardcodedGcpServiceAccount", `
package test

val privateKey = """
    -----BEGIN PRIVATE KEY-----
    abc123
    -----END PRIVATE KEY-----
""".trimIndent()
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestHardcodedGcpServiceAccount_NegativeFileRead(t *testing.T) {
	findings := runRuleByName(t, "HardcodedGcpServiceAccount", `
package test

import java.io.File

val serviceAccount = File("service-account.json").readText()
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestHardcodedGcpServiceAccount_IgnoresPemAndJSONPaths(t *testing.T) {
	cases := []string{"service-account.json", "private-key.pem"}
	code := `
package test

val secret = """
    {"type": "service_account", "project_id": "my-proj"}
    -----BEGIN PRIVATE KEY-----
""".trimIndent()
`

	for _, filename := range cases {
		t.Run(filename, func(t *testing.T) {
			findings := runRuleByNameOnPath(t, "HardcodedGcpServiceAccount", filename, code)
			if len(findings) != 0 {
				t.Fatalf("expected 0 findings for %s, got %d", filename, len(findings))
			}
		})
	}
}
