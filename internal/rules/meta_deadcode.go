// Descriptor metadata for internal/rules/deadcode.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *DeadCodeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DeadCode",
		RuleSet:       "dead-code",
		Severity:      "warning",
		Description:   "Detects public or internal symbols that are never referenced from any other file.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}
