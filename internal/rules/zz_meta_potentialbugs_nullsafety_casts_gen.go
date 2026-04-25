// Descriptor metadata for internal/rules/potentialbugs_nullsafety_casts.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *CastNullableToNonNullableTypeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "CastNullableToNonNullableType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects casting a nullable expression to a non-nullable type using 'as Type'.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *CastToNullableTypeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "CastToNullableType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects casts to nullable types like 'as Type?' which always succeed and may hide bugs.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *UnsafeCastRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnsafeCast",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects non-safe casts using 'as Type' that may throw ClassCastException at runtime.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}
