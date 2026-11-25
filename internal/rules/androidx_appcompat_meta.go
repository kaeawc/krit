package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *UseAndroidAlphaRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseAndroidAlpha", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseAppTintRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseAppTint", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseSwitchCompatOrMaterialXMLRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseSwitchCompatOrMaterialXml", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseCompatTextViewDrawableXMLRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseCompatTextViewDrawableXml", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UsingOnClickInXMLRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UsingOnClickInXml", RuleSet: "android-lint", DefaultActive: true}
}
