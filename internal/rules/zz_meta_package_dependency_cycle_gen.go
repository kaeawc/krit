// Descriptor metadata for internal/rules/package_dependency_cycle.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *PackageDependencyCycleRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "PackageDependencyCycle",
		RuleSet:       "architecture",
		Severity:      "info",
		Description:   "Detects cycles in the package-level import graph within a single Gradle module.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
