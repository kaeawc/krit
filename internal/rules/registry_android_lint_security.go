package rules

// registerAndroidLintSecurityPorts registers the Security checks ported from Android Lint 32.4.1 in
// android_lint_security_*.go.
func registerAndroidLintSecurityPorts() {
	bidi := &BidiSpoofingRule{AndroidRule: lintAndroidRule("BidiSpoofing", "Bidirectional text spoofing", ALCSecurity, ALSError, 9)}
	registerLintRule(bidi.AndroidRule, bidi, bidi.Confidence(), bidi.check, lintSecurityTextNodeTypes, 0, nil)
	auth := &AuthLeakRule{AndroidRule: lintAndroidRule("AuthLeak", "Code might contain an auth leak", ALCSecurity, ALSWarning, 6)}
	registerLintRule(auth.AndroidRule, auth, auth.Confidence(), auth.check, []string{"string_literal"}, 0, nil)
	secret := &SecretInSourceRule{AndroidRule: lintAndroidRule("SecretInSource", "Secret in source code", ALCSecurity, ALSWarning, 9)}
	registerLintRule(secret.AndroidRule, secret, secret.Confidence(), secret.check,
		[]string{"call_expression", "object_creation_expression"}, 0, nil)
}
