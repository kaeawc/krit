// Descriptor metadata for internal/rules/deadcode.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *DeadCodeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DeadCode",
		RuleSet:       "dead-code",
		DefaultActive: false,
		FixLevel:      "semantic",
	}
}
