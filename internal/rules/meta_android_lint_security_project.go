// Descriptor metadata for the Android Lint Security ports in android_lint_security_project.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *TileProviderPermissionsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TileProviderPermissions", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnsafeNativeCodeLocationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeNativeCodeLocation", RuleSet: "android-lint", DefaultActive: true}
}
