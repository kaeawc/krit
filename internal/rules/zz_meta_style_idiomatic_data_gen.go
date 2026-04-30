// Descriptor metadata for internal/rules/style_idiomatic_data.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AlsoCouldBeApplyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AlsoCouldBeApply",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects .also {} blocks with multiple it. references that could use .apply {} instead.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *EqualsNullCallRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EqualsNullCall",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects .equals(null) calls that should use == null instead.",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *ExplicitCollectionElementAccessMethodRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExplicitCollectionElementAccessMethod",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects explicit .get() and .set() calls that should use index operator syntax.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseArrayLiteralsInAnnotationsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseArrayLiteralsInAnnotations",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects arrayOf() calls in annotations that should use array literal [] syntax.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseDataClassRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseDataClass",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects classes with only properties in the constructor that could be data classes.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowVars",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Allow classes with var properties.",
				Apply:       func(target interface{}, value interface{}) { target.(*UseDataClassRule).AllowVars = value.(bool) },
			},
		},
	}
}

func (r *UseIfEmptyOrIfBlankRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseIfEmptyOrIfBlank",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects manual isEmpty/isBlank checks that could use .ifEmpty {} or .ifBlank {} instead.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseIfInsteadOfWhenRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseIfInsteadOfWhen",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects when expressions with two or fewer branches that could be replaced with if.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "ignoreWhenContainingVariableDeclaration",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore when with variable declarations.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UseIfInsteadOfWhenRule).IgnoreWhenContainingVariableDeclaration = value.(bool)
				},
			},
		},
	}
}

func (r *UseLetRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseLet",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects null checks that could be replaced with ?.let {} scope function.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UseSumOfInsteadOfFlatMapSizeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseSumOfInsteadOfFlatMapSize",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects flatMap/map followed by size/count/sum chains that should use sumOf instead.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}
