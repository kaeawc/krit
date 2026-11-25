// Descriptor metadata for the Android Lint Security port in android_lint_security_pending_intent.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *MutableImplicitPendingIntentRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MutableImplicitPendingIntent", RuleSet: "android-lint", DefaultActive: true}
}
