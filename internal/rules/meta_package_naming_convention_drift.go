// Descriptor metadata for internal/rules/package_naming_convention_drift.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *PackageNamingConventionDriftRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PackageNamingConventionDrift",
		RuleSet:       "architecture",
		DefaultActive: false,
		OptInReason:   api.OptInReasonProjectPolicy,
	}
}
