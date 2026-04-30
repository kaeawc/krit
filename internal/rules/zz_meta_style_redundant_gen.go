// Descriptor metadata for internal/rules/style_redundant.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *OptionalUnitRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OptionalUnit",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects explicit Unit return types and return Unit statements that are redundant.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
	}
}

func (r *RedundantConstructorKeywordRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RedundantConstructorKeyword",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects unnecessary constructor keyword on primary constructors without annotations or visibility modifiers.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
	}
}

func (r *RedundantExplicitTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RedundantExplicitType",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects explicit type annotations that can be inferred from the initializer.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
	}
}

func (r *RedundantVisibilityModifierRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RedundantVisibilityModifier",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects explicit public modifier which is redundant since public is the default visibility in Kotlin.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryBackticksRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnnecessaryBackticks",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects backtick-quoted identifiers that do not require backticks.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryInheritanceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnnecessaryInheritance",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects unnecessary explicit inheritance from Any which is implicit in Kotlin.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryInnerClassRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnnecessaryInnerClass",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects inner classes that do not reference the outer class and could remove the inner modifier.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UnnecessaryParenthesesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnnecessaryParentheses",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects unnecessary parentheses around expressions that add no value.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowForUnclearPrecedence",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Allow parentheses for unclear operator precedence.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnnecessaryParenthesesRule).AllowForUnclearPrecedence = value.(bool)
				},
			},
		},
	}
}

func (r *UselessCallOnNotNullRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UselessCallOnNotNull",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects calls like .orEmpty() or .isNullOrEmpty() on receivers that are already non-null.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}
