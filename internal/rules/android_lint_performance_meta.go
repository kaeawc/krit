package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *UnnecessaryArrayInitRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnnecessaryArrayInit", RuleSet: "android-lint", DefaultActive: true}
}

// ExpensiveAssertion is off by default in Lint.
func (r *ExpensiveAssertionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ExpensiveAssertion", RuleSet: "android-lint", DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated}
}

func (r *AssertionSideEffectRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "AssertionSideEffect", RuleSet: "android-lint", DefaultActive: true}
}

func (r *NotificationTrampolineRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "NotificationTrampoline", RuleSet: "android-lint", DefaultActive: true}
}

func (r *LaunchActivityFromNotificationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "LaunchActivityFromNotification", RuleSet: "android-lint", DefaultActive: true}
}
