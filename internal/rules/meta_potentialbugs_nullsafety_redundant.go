// Descriptor metadata for internal/rules/potentialbugs_nullsafety_redundant.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *NullCheckOnMutablePropertyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NullCheckOnMutableProperty",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
	}
}

func (r *NullableToStringCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NullableToStringCall",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
	}
}

func (r *UnnecessaryNotNullCheckRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryNotNullCheck",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		FixLevel:      "semantic",
	}
}

func (r *UnnecessaryNotNullOperatorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryNotNullOperator",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *UnnecessarySafeCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessarySafeCall",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *UselessElvisOnNonNullRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UselessElvisOnNonNull",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}
