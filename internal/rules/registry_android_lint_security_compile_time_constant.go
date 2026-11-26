package rules

import "github.com/kaeawc/krit/internal/rules/api"

// registerAndroidLintSecurityCompileTimeConstantPorts registers the Android Lint 32.4.1 CompileTimeConstant check in
// android_lint_security_compile_time_constant.go.
func registerAndroidLintSecurityCompileTimeConstantPorts() {
	r := &CompileTimeConstantRule{AndroidRule: lintAndroidRule("CompileTimeConstant", "Compile-time constant required",
		ALCSecurity, ALSError, 8)}
	rule := registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, compileTimeConstantNodeTypes, api.NeedsResolver, nil)
	rule.FileGateWithResolver = ctcFileGate
	rule.LanguageSupport = map[string]api.LanguageSupport{JavaLanguageSupportKey: {
		Status: api.LanguageSupportPartial,
		Reason: "Java callers and callees are checked within one file; callees in other files resolve through the " +
			"Kotlin type index, which records no Java method parameters.",
		Evidence: []string{"internal/rules/android_lint_security_compile_time_constant_test.go"},
	}}
}
