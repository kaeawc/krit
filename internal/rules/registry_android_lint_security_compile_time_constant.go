package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintSecurityCompileTimeConstantPorts registers the Android Lint 32.4.1 CompileTimeConstant check in
// android_lint_security_compile_time_constant.go.
//
// It runs in the cross-file phase: a file's findings can depend on @CompileTimeConstant declarations in other files
// (#1610), and that phase's findings cache is keyed by the code index fingerprint, which covers every file, while
// the per-file analysis cache is keyed by the file's own content only (#1778).
func registerAndroidLintSecurityCompileTimeConstantPorts() {
	r := &CompileTimeConstantRule{AndroidRule: lintAndroidRule("CompileTimeConstant", "Compile-time constant required",
		ALCSecurity, ALSError, 8)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		Needs:      api.NeedsCrossFile | api.NeedsResolver,
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: r.Confidence(), Implementation: r, Check: r.checkProject, NeedsLibraryFacts: true,
		LanguageSupport: map[string]api.LanguageSupport{JavaLanguageSupportKey: {
			Status: api.LanguageSupportPartial,
			Reason: "Java callers and callees are checked within one file; callees in other files resolve through the " +
				"Kotlin type index, which records no Java method parameters.",
			Evidence: []string{"internal/rules/android_lint_security_compile_time_constant_test.go"},
		}},
	})
}
