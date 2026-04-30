// Descriptor metadata for internal/rules/potentialbugs_nullsafety_redundant.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *NullCheckOnMutablePropertyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NullCheckOnMutableProperty",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects null checks on mutable var properties that may be changed by another thread between the check and use.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NullableToStringCallRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NullableToStringCall",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects .toString() calls on nullable receivers that may produce the string \"null\".",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryNotNullCheckRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnnecessaryNotNullCheck",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects unnecessary null checks on expressions that are already non-nullable.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryNotNullOperatorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnnecessaryNotNullOperator",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects the !! operator applied to expressions that are already non-nullable.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *UnnecessarySafeCallRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnnecessarySafeCall",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects the ?. safe-call operator applied to expressions that are already non-nullable.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}
