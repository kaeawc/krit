// Descriptor metadata for internal/rules/style_redundant.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *OptionalUnitRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OptionalUnit",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}

func (r *RedundantConstructorKeywordRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RedundantConstructorKeyword",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}

func (r *RedundantExplicitTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RedundantExplicitType",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}

func (r *RedundantVisibilityModifierRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RedundantVisibilityModifier",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}

func (r *UnnecessaryBackticksRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryBackticks",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}

func (r *UnnecessaryInheritanceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryInheritance",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryInnerClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryInnerClass",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *UnnecessaryParenthesesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnnecessaryParentheses",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[UnnecessaryParenthesesRule]{
				Name:        "allowForUnclearPrecedence",
				Default:     false,
				Description: "Allow parentheses for unclear operator precedence.",
				Apply:       func(r *UnnecessaryParenthesesRule, v bool) { r.AllowForUnclearPrecedence = v },
			}),
		},
	}
}

func (r *UselessCallOnNotNullRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UselessCallOnNotNull",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}
