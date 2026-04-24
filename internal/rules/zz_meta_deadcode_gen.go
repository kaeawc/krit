// Descriptor metadata for internal/rules/deadcode.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *DeadCodeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DeadCode",
		RuleSet:       "dead-code",
		Severity:      "warning",
		Description:   "Detects public or internal symbols that are never referenced from any other file.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}
