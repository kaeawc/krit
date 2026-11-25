// Descriptor metadata for the Android Lint Security ports that join sources with the manifests.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *ProviderReadPermissionOnlyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ProviderReadPermissionOnly", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnsafeImplicitIntentLaunchRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeImplicitIntentLaunch", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnsafeIntentLaunchRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeIntentLaunch", RuleSet: "android-lint", DefaultActive: true}
}
