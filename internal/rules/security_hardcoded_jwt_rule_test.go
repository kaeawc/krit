package rules_test

import (
	"strings"
	"testing"
)

const sampleHardcodedJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

func TestHardcodedJwt_Positive_KotlinLiteral(t *testing.T) {
	findings := runRuleByName(t, "HardcodedJwt", `
val token = "`+sampleHardcodedJWT+`"
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "JWT") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestHardcodedJwt_Positive_RawString(t *testing.T) {
	findings := runRuleByName(t, "HardcodedJwt", "val token = \"\"\""+sampleHardcodedJWT+"\"\"\"")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestHardcodedJwt_Negative_NonJwtLiteral(t *testing.T) {
	findings := runRuleByName(t, "HardcodedJwt", `
val s = "just a normal string with dots.in.it"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestHardcodedJwt_Negative_PlaceholderToken(t *testing.T) {
	findings := runRuleByName(t, "HardcodedJwt", `
val token = "eyJhbGciOiJIUzI1NiJ9.YOUR_TOKEN_HERE.signaturebody"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestHardcodedJwt_Negative_DynamicInterpolation(t *testing.T) {
	findings := runRuleByName(t, "HardcodedJwt", `
fun call(jwt: String) {
    val auth = "$jwt"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestHardcodedJwt_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "HardcodedJwt", `
class Auth {
    static final String TOKEN = "`+sampleHardcodedJWT+`";
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
	}
}
