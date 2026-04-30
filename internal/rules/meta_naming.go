// Descriptor metadata for internal/rules/naming.go.

package rules

import (
	"regexp"

	"github.com/kaeawc/krit/internal/rules/v2"
)

var _ = regexp.MustCompile

func (r *BooleanPropertyNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "BooleanPropertyNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects Boolean properties that do not start with an allowed prefix like is, has, or are.",
		DefaultActive: false,
		FixLevel:      "semantic",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedPattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for allowed non-standard Boolean property names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*BooleanPropertyNamingRule).AllowedPattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *ClassNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ClassNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects class names that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "classPattern",
				Type:        v2.OptRegex,
				Default:     "[A-Z][a-zA-Z0-9]*",
				Description: "Regex pattern for class names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ClassNamingRule).Pattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *ConstructorParameterNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ConstructorParameterNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects constructor val/var parameter names that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "excludeClassPattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for classes to exclude from this rule.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ConstructorParameterNamingRule).ExcludeClassPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "parameterPattern",
				Type:        v2.OptRegex,
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for constructor parameter names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ConstructorParameterNamingRule).Pattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "privateParameterPattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for private constructor parameter names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ConstructorParameterNamingRule).PrivateParameterPattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *EnumNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EnumNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects enum entry names that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "enumEntryPattern",
				Type:        v2.OptRegex,
				Default:     "[A-Z][_a-zA-Z0-9]*",
				Description: "Regex pattern for enum entry names.",
				Apply:       func(target interface{}, value interface{}) { target.(*EnumNamingRule).Pattern = value.(*regexp.Regexp) },
			},
		},
	}
}

func (r *ForbiddenClassNameRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ForbiddenClassName",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects class names that match a configured list of disallowed names.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "forbiddenName",
				Type:        v2.OptStringList,
				Default:     []string{"Manager", "Helper", "Util", "Utils"},
				Description: "List of forbidden class names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ForbiddenClassNameRule).ForbiddenNames = value.([]string)
				},
			},
		},
	}
}

func (r *FunctionNameMaxLengthRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FunctionNameMaxLength",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects function names that exceed the configured maximum length.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "maximumFunctionNameLength",
				Aliases:     []string{"maxLength"},
				Type:        v2.OptInt,
				Default:     30,
				Description: "Maximum allowed function name length.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FunctionNameMaxLengthRule).MaxLength = value.(int)
				},
			},
		},
	}
}

func (r *FunctionNameMinLengthRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FunctionNameMinLength",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects function names that are shorter than the configured minimum length.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "minimumFunctionNameLength",
				Aliases:     []string{"minLength"},
				Type:        v2.OptInt,
				Default:     3,
				Description: "Minimum allowed function name length.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FunctionNameMinLengthRule).MinLength = value.(int)
				},
			},
		},
	}
}

func (r *FunctionNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FunctionNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects function names that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "excludeClassPattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for classes to exclude from this rule.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FunctionNamingRule).ExcludeClassPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "functionPattern",
				Type:        v2.OptRegex,
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for function names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FunctionNamingRule).Pattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "ignoreAnnotated",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Annotations that suppress this rule.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FunctionNamingRule).IgnoreAnnotated = value.([]string)
				},
			},
		},
	}
}

func (r *FunctionParameterNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FunctionParameterNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects function parameter names that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "excludeClassPattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for classes to exclude from this rule.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FunctionParameterNamingRule).ExcludeClassPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "parameterPattern",
				Type:        v2.OptRegex,
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for function parameter names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FunctionParameterNamingRule).Pattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *InvalidPackageDeclarationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InvalidPackageDeclaration",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects package declarations that do not match the file directory structure.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "requireRootInDeclaration",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Require rootPackage in package declaration.",
				Apply: func(target interface{}, value interface{}) {
					target.(*InvalidPackageDeclarationRule).RequireRootInDeclaration = value.(bool)
				},
			},
			{
				Name:        "rootPackage",
				Type:        v2.OptString,
				Default:     "",
				Description: "Root package prefix to require.",
				Apply: func(target interface{}, value interface{}) {
					target.(*InvalidPackageDeclarationRule).RootPackage = value.(string)
				},
			},
		},
	}
}

func (r *LambdaParameterNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LambdaParameterNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects lambda parameter names that do not match the expected naming pattern.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "parameterPattern",
				Type:        v2.OptRegex,
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for lambda parameter names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*LambdaParameterNamingRule).Pattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *MatchingDeclarationNameRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MatchingDeclarationName",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects files where the single top-level declaration name does not match the filename.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "multiplatformTargets",
				Type:        v2.OptStringList,
				Default:     []string{"ios", "android", "js", "jvm", "native", "iosArm64", "iosX64", "macosX64", "mingwX64", "linuxX64"},
				Description: "Multiplatform target suffixes to strip from filename.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MatchingDeclarationNameRule).MultiplatformTargets = value.([]string)
				},
			},
			{
				Name:        "mustBeFirst",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Declaration must be first in file.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MatchingDeclarationNameRule).MustBeFirst = value.(bool)
				},
			},
		},
	}
}

func (r *MemberNameEqualsClassNameRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MemberNameEqualsClassName",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects class members whose name is the same as the containing class name.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "ignoreOverridden",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Ignore overridden members.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MemberNameEqualsClassNameRule).IgnoreOverridden = value.(bool)
				},
			},
		},
	}
}

func (r *NoNameShadowingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NoNameShadowing",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects inner declarations that shadow an outer declaration with the same name.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NonBooleanPropertyPrefixedWithIsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NonBooleanPropertyPrefixedWithIs",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects non-Boolean properties whose name starts with the is prefix.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *ObjectPropertyNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ObjectPropertyNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects property names inside object declarations that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "constantPattern",
				Type:        v2.OptRegex,
				Default:     "[A-Z][_A-Z0-9]*",
				Description: "Regex pattern for constant properties in objects.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ObjectPropertyNamingRule).ConstPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "privatePropertyPattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for private properties in objects.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ObjectPropertyNamingRule).PrivatePropertyPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "propertyPattern",
				Type:        v2.OptRegex,
				Default:     "[a-z][A-Za-z0-9]*",
				Description: "Regex pattern for non-constant properties in objects.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ObjectPropertyNamingRule).PropertyPattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *PackageNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PackageNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects package names that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "packagePattern",
				Type:        v2.OptRegex,
				Default:     "[a-z]+(\\.[a-z][A-Za-z0-9]*)*",
				Description: "Regex pattern for package names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*PackageNamingRule).Pattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *TopLevelPropertyNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TopLevelPropertyNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects top-level property names that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "constantPattern",
				Type:        v2.OptRegex,
				Default:     "[A-Z][_A-Za-z0-9]*",
				Description: "Regex pattern for top-level constant properties.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TopLevelPropertyNamingRule).ConstPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "privatePropertyPattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for private top-level properties.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TopLevelPropertyNamingRule).PrivatePropertyPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "propertyPattern",
				Type:        v2.OptRegex,
				Default:     "[a-z][A-Za-z0-9]*",
				Description: "Regex pattern for top-level non-constant properties.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TopLevelPropertyNamingRule).PropertyPattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *VariableMaxLengthRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "VariableMaxLength",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects variable names that exceed the configured maximum length.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "maximumVariableNameLength",
				Aliases:     []string{"maxLength"},
				Type:        v2.OptInt,
				Default:     64,
				Description: "Maximum allowed variable name length.",
				Apply:       func(target interface{}, value interface{}) { target.(*VariableMaxLengthRule).MaxLength = value.(int) },
			},
		},
	}
}

func (r *VariableMinLengthRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "VariableMinLength",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects variable names that are shorter than the configured minimum length.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "minimumVariableNameLength",
				Aliases:     []string{"minLength"},
				Type:        v2.OptInt,
				Default:     2,
				Description: "Minimum allowed variable name length.",
				Apply:       func(target interface{}, value interface{}) { target.(*VariableMinLengthRule).MinLength = value.(int) },
			},
		},
	}
}

func (r *VariableNamingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "VariableNaming",
		RuleSet:       "naming",
		Severity:      "warning",
		Description:   "Detects local variable names that do not match the expected naming pattern.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "excludeClassPattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex for classes to exclude from this rule.",
				Apply: func(target interface{}, value interface{}) {
					target.(*VariableNamingRule).ExcludeClassPattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "privateVariablePattern",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex pattern for private variable names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*VariableNamingRule).PrivateVariablePattern = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "variablePattern",
				Type:        v2.OptRegex,
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for variable names.",
				Apply: func(target interface{}, value interface{}) {
					target.(*VariableNamingRule).Pattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}
