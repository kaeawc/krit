// Descriptor metadata for internal/rules/style_classes.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AbstractClassCanBeConcreteClassRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AbstractClassCanBeConcreteClass",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects abstract classes that have no abstract members and could be made concrete.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *AbstractClassCanBeInterfaceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AbstractClassCanBeInterface",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects abstract classes with no state that could be converted to interfaces.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *ClassOrderingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ClassOrdering",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects class members that are not in the conventional ordering of properties, init blocks, constructors, functions, and companion objects.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DataClassContainsFunctionsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DataClassContainsFunctions",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects data classes that contain function members.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "conversionFunctionPrefix",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Prefixes for allowed conversion functions.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DataClassContainsFunctionsRule).ConversionFunctionPrefix = value.([]string)
				},
			},
		},
	}
}

func (r *DataClassShouldBeImmutableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DataClassShouldBeImmutable",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects data class properties declared as var instead of val.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *NestedClassesVisibilityRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NestedClassesVisibility",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects nested classes with explicit public modifier inside internal parent classes.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ObjectLiteralToLambdaRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ObjectLiteralToLambda",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects object literals implementing a single method that could be converted to a lambda.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OptionalAbstractKeywordRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OptionalAbstractKeyword",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects redundant abstract modifier on interface members where it is implied.",
		DefaultActive: true,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
	}
}

func (r *ProtectedMemberInFinalClassRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ProtectedMemberInFinalClass",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects protected members in final classes where they should be private.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *SerialVersionUIDInSerializableClassRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SerialVersionUIDInSerializableClass",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects Serializable classes that are missing a serialVersionUID field.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *UtilityClassWithPublicConstructorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UtilityClassWithPublicConstructor",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects utility classes that have a public constructor instead of a private one.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}
