package rules

// registerAndroidLintSecurityPendingIntentPorts registers the Android Lint 32.4.1 MutableImplicitPendingIntent check
// in android_lint_security_pending_intent.go.
func registerAndroidLintSecurityPendingIntentPorts() {
	r := &MutableImplicitPendingIntentRule{AndroidRule: lintAndroidRule("MutableImplicitPendingIntent",
		"Mutable Implicit PendingIntent is disallowed", ALCSecurity, ALSError, 5)}
	registerLintCallRule(r.AndroidRule, r, r.Confidence(), r.check)
}
