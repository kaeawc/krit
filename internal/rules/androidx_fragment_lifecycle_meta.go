package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *FragmentLiveDataObserveRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "FragmentLiveDataObserve", RuleSet: "android-lint", DefaultActive: true}
}

func (r *FragmentBackPressedCallbackRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "FragmentBackPressedCallback", RuleSet: "android-lint", DefaultActive: true}
}

func (r *FragmentAddMenuProviderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "FragmentAddMenuProvider", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnsafeRepeatOnLifecycleDetectorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeRepeatOnLifecycleDetector", RuleSet: "android-lint", DefaultActive: true}
}
