// Descriptor metadata for internal/rules/module_dependency_cycle.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *ModuleDependencyCycleRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ModuleDependencyCycle",
		RuleSet:       "architecture",
		Severity:      "warning",
		Description:   "Detects cycles in the Gradle module dependency graph (e.g. :a → :b → :c → :a).",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
