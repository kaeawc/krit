package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *UseRequireInsteadOfGetRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseRequireInsteadOfGet", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseGetLayoutInflaterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseGetLayoutInflater", RuleSet: "android-lint", DefaultActive: true}
}

func (r *DialogFragmentCallbacksDetectorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DialogFragmentCallbacksDetector", RuleSet: "android-lint", DefaultActive: true}
}

func (r *DetachAndAttachSameFragmentRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DetachAndAttachSameFragment", RuleSet: "android-lint", DefaultActive: true}
}
