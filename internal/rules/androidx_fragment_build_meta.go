package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *FragmentGradleConfigurationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "FragmentGradleConfiguration", RuleSet: "android-lint", DefaultActive: true}
}

func (r *FragmentTagUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "FragmentTagUsage", RuleSet: "android-lint", DefaultActive: true}
}
