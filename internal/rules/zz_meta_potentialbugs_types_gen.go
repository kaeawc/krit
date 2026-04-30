// Descriptor metadata for internal/rules/potentialbugs_types.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AvoidReferentialEqualityRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AvoidReferentialEquality",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects usage of referential equality operators === or !== which compare object identity instead of value.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "forbiddenTypePatterns",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Type patterns where === is forbidden.",
				Apply: func(target interface{}, value interface{}) {
					target.(*AvoidReferentialEqualityRule).ForbiddenTypePatterns = value.([]string)
				},
			},
		},
	}
}

func (r *CharArrayToStringCallRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CharArrayToStringCall",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects charArray.toString() calls that return the array's address instead of its content.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *DontDowncastCollectionTypesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DontDowncastCollectionTypes",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects downcasting read-only collection types to mutable variants like 'as MutableList'.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *DoubleMutabilityForCollectionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DoubleMutabilityForCollection",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects var declarations with mutable collection types, creating double mutability.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "mutableTypes",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Additional mutable types to check.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DoubleMutabilityForCollectionRule).MutableTypes = value.([]string)
				},
			},
		},
	}
}

func (r *ElseCaseInsteadOfExhaustiveWhenRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ElseCaseInsteadOfExhaustiveWhen",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects when expressions on sealed classes or enums that use an else branch instead of exhaustive matching.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *EqualsAlwaysReturnsTrueOrFalseRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EqualsAlwaysReturnsTrueOrFalse",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects equals() implementations that always return true or always return false.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *EqualsWithHashCodeExistRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EqualsWithHashCodeExist",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects classes that override equals() without hashCode() or vice versa.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ImplicitUnitReturnTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ImplicitUnitReturnType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects block-body functions that implicitly return Unit without an explicit return type.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *WrongEqualsTypeParameterRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WrongEqualsTypeParameter",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects equals() with a parameter type other than Any?, which does not properly override the contract.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}
