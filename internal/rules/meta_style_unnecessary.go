// Descriptor metadata for internal/rules/style_unnecessary.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *RedundantHigherOrderMapUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RedundantHigherOrderMapUsage",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryAnyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryAny",
		RuleSet:       "style",
		DefaultActive: false,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryApplyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryApply",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryBracesAroundTrailingLambdaRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryBracesAroundTrailingLambda",
		RuleSet:       "style",
		DefaultActive: false,
		FixLevel:      "cosmetic",
	}
}

func (r *UnnecessaryFilterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryFilter",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryFullyQualifiedNameRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryFullyQualifiedName",
		RuleSet:       "style",
		DefaultActive: false,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryLetRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryLet",
		RuleSet:       "style",
		DefaultActive: false,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryReversedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryReversed",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}
