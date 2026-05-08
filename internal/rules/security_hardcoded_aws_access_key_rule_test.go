package rules_test

import (
	"strings"
	"testing"
)

const sampleHardcodedAwsKey = "AKIA1234567890ABCDEF"

func TestHardcodedAwsAccessKey_Positive_KotlinLiteral(t *testing.T) {
	findings := runRuleByName(t, "HardcodedAwsAccessKey", `
val key = "`+sampleHardcodedAwsKey+`"
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "AWS") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestHardcodedAwsAccessKey_Positive_RawString(t *testing.T) {
	findings := runRuleByName(t, "HardcodedAwsAccessKey", "val key = \"\"\""+sampleHardcodedAwsKey+"\"\"\"")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestHardcodedAwsAccessKey_Negative_Lowercase(t *testing.T) {
	findings := runRuleByName(t, "HardcodedAwsAccessKey", `
val key = "akia1234567890abcdef"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestHardcodedAwsAccessKey_Negative_DocsExamplePlaceholder(t *testing.T) {
	// AWS docs use the literal `EXAMPLE` suffix to mark fake credentials.
	findings := runRuleByName(t, "HardcodedAwsAccessKey", `
val key = "AKIAIOSFODNN7EXAMPLE"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestHardcodedAwsAccessKey_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "HardcodedAwsAccessKey", `
class Aws {
    static final String KEY = "`+sampleHardcodedAwsKey+`";
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
	}
}
