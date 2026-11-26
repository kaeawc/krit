// Descriptor metadata for the Android Lint Security port in android_lint_security_compile_time_constant.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *CompileTimeConstantRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "CompileTimeConstant", RuleSet: "android-lint", DefaultActive: true}
}
