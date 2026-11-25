package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *InvalidFragmentVersionForActivityResultRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidFragmentVersionForActivityResult", RuleSet: "android-lint", DefaultActive: true}
}

func (r *InvalidUseOfOnBackPressedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidUseOfOnBackPressed", RuleSet: "android-lint", DefaultActive: true}
}
