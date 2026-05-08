// Descriptor metadata for internal/rules/package_dependency_cycle.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *PackageDependencyCycleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PackageDependencyCycle",
		RuleSet:       "architecture",
		DefaultActive: false,
	}
}
