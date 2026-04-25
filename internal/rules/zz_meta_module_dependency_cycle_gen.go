// Descriptor metadata for internal/rules/module_dependency_cycle.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *ModuleDependencyCycleRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ModuleDependencyCycle",
		RuleSet:       "architecture",
		Severity:      "warning",
		Description:   "Detects cycles in the Gradle module dependency graph (e.g. :a → :b → :c → :a).",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
