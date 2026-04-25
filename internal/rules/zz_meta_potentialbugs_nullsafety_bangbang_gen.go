// Descriptor metadata for internal/rules/potentialbugs_nullsafety_bangbang.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *MapGetWithNotNullAssertionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MapGetWithNotNullAssertionOperator",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects map[key]!! usage and suggests getValue() or safe alternatives.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *UnsafeCallOnNullableTypeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnsafeCallOnNullableType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects usage of the !! not-null assertion operator which may throw NullPointerException.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
