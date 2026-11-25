// Descriptor metadata for the Android Lint Security ports in android_lint_security_*.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *BidiSpoofingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "BidiSpoofing", RuleSet: "android-lint", DefaultActive: true}
}

func (r *AuthLeakRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "AuthLeak", RuleSet: "android-lint", DefaultActive: true}
}

func (r *SecretInSourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SecretInSource", RuleSet: "android-lint", DefaultActive: true}
}
