// Descriptor metadata for the Android Lint Security port in android_lint_security_compile_time_constant.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *CompileTimeConstantRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "CompileTimeConstant", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Callees declared in other files resolve only for Kotlin callers and Kotlin source declarations (top-level functions, members of the receiver's type and its supertypes, and operator functions); Java methods in other files and library callees are not resolved (#1767).",
		}}
}
