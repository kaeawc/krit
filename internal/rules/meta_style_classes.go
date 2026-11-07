// Descriptor metadata for internal/rules/style_classes.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AbstractClassCanBeConcreteClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AbstractClassCanBeConcreteClass",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *AbstractClassCanBeInterfaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AbstractClassCanBeInterface",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *ClassOrderingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ClassOrdering",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *DataClassContainsFunctionsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DataClassContainsFunctions",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[DataClassContainsFunctionsRule]{
				Name:        "conversionFunctionPrefix",
				Description: "Prefixes for allowed conversion functions.",
				Apply:       func(r *DataClassContainsFunctionsRule, v []string) { r.ConversionFunctionPrefix = v },
			}),
			api.BoolOption(api.BoolOptionSpec[DataClassContainsFunctionsRule]{
				Name:        "allowOperators",
				Default:     false,
				Description: "Allow operator functions in data classes.",
				Apply:       func(r *DataClassContainsFunctionsRule, v bool) { r.AllowOperators = v },
			}),
		},
	}
}

func (r *DataClassShouldBeImmutableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DataClassShouldBeImmutable",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[DataClassShouldBeImmutableRule]{
				Name:        "reportMutableCollections",
				Default:     true,
				Description: "Also report a val whose type is a mutable collection. krit extension; detekt only reports var properties, so detekt's defaults set this to false.",
				Apply:       func(r *DataClassShouldBeImmutableRule, v bool) { r.ReportMutableCollections = v },
			}),
		},
	}
}

func (r *NestedClassesVisibilityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NestedClassesVisibility",
		RuleSet:       "style",
		DefaultActive: true,
	}
}

func (r *ObjectLiteralToLambdaRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ObjectLiteralToLambda",
		RuleSet:       "style",
		DefaultActive: true,
	}
}

func (r *OptionalAbstractKeywordRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OptionalAbstractKeyword",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "cosmetic",
	}
}

func (r *ProtectedMemberInFinalClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ProtectedMemberInFinalClass",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *SerialVersionUIDInSerializableClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SerialVersionUIDInSerializableClass",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *UtilityClassWithPublicConstructorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UtilityClassWithPublicConstructor",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}
