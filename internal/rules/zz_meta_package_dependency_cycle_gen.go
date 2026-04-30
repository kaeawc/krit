// Descriptor metadata for internal/rules/package_dependency_cycle.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *PackageDependencyCycleRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PackageDependencyCycle",
		RuleSet:       "architecture",
		Severity:      "info",
		Description:   "Detects cycles in the package-level import graph within a single Gradle module.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
