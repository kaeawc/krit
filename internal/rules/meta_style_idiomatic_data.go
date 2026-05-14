// Descriptor metadata for internal/rules/style_idiomatic_data.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AlsoCouldBeApplyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AlsoCouldBeApply",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
	}
}

func (r *EqualsNullCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EqualsNullCall",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

func (r *ExplicitCollectionElementAccessMethodRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExplicitCollectionElementAccessMethod",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *UseArrayLiteralsInAnnotationsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseArrayLiteralsInAnnotations",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *UseDataClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseDataClass",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[UseDataClassRule]{
				Name:        "allowVars",
				Default:     false,
				Description: "Allow classes with var properties.",
				Apply:       func(r *UseDataClassRule, v bool) { r.AllowVars = v },
			}),
		},
	}
}

func (r *UseIfEmptyOrIfBlankRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseIfEmptyOrIfBlank",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *UseIfInsteadOfWhenRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseIfInsteadOfWhen",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[UseIfInsteadOfWhenRule]{
				Name:        "ignoreWhenContainingVariableDeclaration",
				Default:     false,
				Description: "Ignore when with variable declarations.",
				Apply:       func(r *UseIfInsteadOfWhenRule, v bool) { r.IgnoreWhenContainingVariableDeclaration = v },
			}),
		},
	}
}

func (r *UseLetRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseLet",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *UseSumOfInsteadOfFlatMapSizeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseSumOfInsteadOfFlatMapSize",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}
