// Descriptor metadata for internal/rules/potentialbugs_nullsafety_casts.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *CastNullableToNonNullableTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CastNullableToNonNullableType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects casting a nullable expression to a non-nullable type using 'as Type'.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *CastToNullableTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CastToNullableType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects casts to nullable types like 'as Type?' which always succeed and may hide bugs.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *UnsafeCastRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnsafeCast",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects casts that Kotlin reports can never succeed.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
	}
}
