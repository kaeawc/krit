// Descriptor metadata for internal/rules/potentialbugs_nullsafety_bangbang.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *MapGetWithNotNullAssertionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MapGetWithNotNullAssertionOperator",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects map[key]!! usage and suggests getValue() or safe alternatives.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *UnsafeCallOnNullableTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnsafeCallOnNullableType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects usage of the !! not-null assertion operator which may throw NullPointerException.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
