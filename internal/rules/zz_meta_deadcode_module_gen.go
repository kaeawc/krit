// Descriptor metadata for internal/rules/deadcode_module.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *ModuleDeadCodeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ModuleDeadCode",
		RuleSet:       "dead-code",
		Severity:      "warning",
		Description:   "Detects dead code with module-boundary awareness, categorizing symbols as truly dead or could-be-internal.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
