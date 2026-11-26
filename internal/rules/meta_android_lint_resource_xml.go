package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *StringEscapingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "StringEscaping", RuleSet: "android-lint", DefaultActive: true}
}

func (r *InvalidNavigationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidNavigation", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"An included graph is looked up in every res directory of the scan; Lint looks only in the module unless it runs a global analysis.",
		}}
}

func (r *ValidRestrictionsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ValidRestrictions", RuleSet: "android-lint", DefaultActive: true}
}

func (r *RelativeClassResolutionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RelativeClassResolution", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The namespace and applicationId come from literal `namespace` / `applicationId` assignments in the module's build script (or the manifest package); values set through variables, flavors or convention plugins are not seen, and the check is skipped without a namespace.",
		}}
}
