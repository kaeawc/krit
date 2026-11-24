package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *UseSupportActionBarRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseSupportActionBar", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseCompatLoadingForColorStateListsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseCompatLoadingForColorStateLists", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseCompatLoadingForDrawablesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseCompatLoadingForDrawables", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseCompatTextViewDrawableApisRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseCompatTextViewDrawableApis", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseSwitchCompatOrMaterialCodeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseSwitchCompatOrMaterialCode", RuleSet: "android-lint", DefaultActive: true}
}
