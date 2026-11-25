package rules

// registerAndroidLintSecurityCompileTimeConstantPorts registers the Android Lint 32.4.1 CompileTimeConstant check in
// android_lint_security_compile_time_constant.go.
func registerAndroidLintSecurityCompileTimeConstantPorts() {
	r := &CompileTimeConstantRule{AndroidRule: lintAndroidRule("CompileTimeConstant", "Compile-time constant required",
		ALCSecurity, ALSError, 8)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, compileTimeConstantNodeTypes, 0, nil)
}
