// Descriptor metadata for internal/rules/potentialbugs_types.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AvoidReferentialEqualityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AvoidReferentialEquality",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[AvoidReferentialEqualityRule]{
				Name:        "forbiddenTypePatterns",
				Description: "Type patterns where === is forbidden.",
				Apply:       func(r *AvoidReferentialEqualityRule, v []string) { r.ForbiddenTypePatterns = v },
			}),
		},
	}
}

func (r *CharArrayToStringCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CharArrayToStringCall",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *DontDowncastCollectionTypesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DontDowncastCollectionTypes",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
	}
}

func (r *DoubleMutabilityForCollectionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DoubleMutabilityForCollection",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[DoubleMutabilityForCollectionRule]{
				Name:        "mutableTypes",
				Description: "Additional mutable types to check.",
				Apply:       func(r *DoubleMutabilityForCollectionRule, v []string) { r.MutableTypes = v },
			}),
		},
	}
}

func (r *ElseCaseInsteadOfExhaustiveWhenRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ElseCaseInsteadOfExhaustiveWhen",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *EqualsAlwaysReturnsTrueOrFalseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EqualsAlwaysReturnsTrueOrFalse",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *EqualsWithHashCodeExistRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EqualsWithHashCodeExist",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *ImplicitUnitReturnTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ImplicitUnitReturnType",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *WrongEqualsTypeParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongEqualsTypeParameter",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}
