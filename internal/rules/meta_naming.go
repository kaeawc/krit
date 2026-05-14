// Descriptor metadata for internal/rules/naming.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var _ = regexp.MustCompile

func (r *BooleanPropertyNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BooleanPropertyNaming",
		RuleSet:       "naming",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[BooleanPropertyNamingRule]{
				Name:        "allowedPattern",
				Default:     "",
				Description: "Regex for allowed non-standard Boolean property names.",
				Apply:       func(r *BooleanPropertyNamingRule, v *regexp.Regexp) { r.AllowedPattern = v },
			}),
		},
	}
}

func (r *ClassNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ClassNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[ClassNamingRule]{
				Name:        "classPattern",
				Default:     "[A-Z][a-zA-Z0-9]*",
				Description: "Regex pattern for class names.",
				Apply:       func(r *ClassNamingRule, v *regexp.Regexp) { r.Pattern = v },
			}),
		},
	}
}

func (r *ConstructorParameterNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ConstructorParameterNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[ConstructorParameterNamingRule]{
				Name:        "excludeClassPattern",
				Default:     "",
				Description: "Regex for classes to exclude from this rule.",
				Apply:       func(r *ConstructorParameterNamingRule, v *regexp.Regexp) { r.ExcludeClassPattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[ConstructorParameterNamingRule]{
				Name:        "parameterPattern",
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for constructor parameter names.",
				Apply:       func(r *ConstructorParameterNamingRule, v *regexp.Regexp) { r.Pattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[ConstructorParameterNamingRule]{
				Name:        "privateParameterPattern",
				Default:     "",
				Description: "Regex for private constructor parameter names.",
				Apply:       func(r *ConstructorParameterNamingRule, v *regexp.Regexp) { r.PrivateParameterPattern = v },
			}),
		},
	}
}

func (r *EnumNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EnumNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[EnumNamingRule]{
				Name:        "enumEntryPattern",
				Default:     "[A-Z][_a-zA-Z0-9]*",
				Description: "Regex pattern for enum entry names.",
				Apply:       func(r *EnumNamingRule, v *regexp.Regexp) { r.Pattern = v },
			}),
		},
	}
}

func (r *ForbiddenClassNameRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ForbiddenClassName",
		RuleSet:       "naming",
		DefaultActive: false,
		OptInReason:   api.OptInReasonRequiresUserConfig,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ForbiddenClassNameRule]{
				Name:        "forbiddenName",
				Default:     []string{"Manager", "Helper", "Util", "Utils"},
				Description: "List of forbidden class names.",
				Apply:       func(r *ForbiddenClassNameRule, v []string) { r.ForbiddenNames = v },
			}),
		},
	}
}

func (r *FunctionNameMaxLengthRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FunctionNameMaxLength",
		RuleSet:       "naming",
		DefaultActive: false,
		OptInReason:   api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[FunctionNameMaxLengthRule]{
				Name:        "maximumFunctionNameLength",
				Aliases:     []string{"maxLength"},
				Default:     30,
				Description: "Maximum allowed function name length.",
				Apply:       func(r *FunctionNameMaxLengthRule, v int) { r.MaxLength = v },
			}),
		},
	}
}

func (r *FunctionNameMinLengthRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FunctionNameMinLength",
		RuleSet:       "naming",
		DefaultActive: false,
		OptInReason:   api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[FunctionNameMinLengthRule]{
				Name:        "minimumFunctionNameLength",
				Aliases:     []string{"minLength"},
				Default:     3,
				Description: "Minimum allowed function name length.",
				Apply:       func(r *FunctionNameMinLengthRule, v int) { r.MinLength = v },
			}),
		},
	}
}

func (r *FunctionNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FunctionNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[FunctionNamingRule]{
				Name:        "excludeClassPattern",
				Default:     "",
				Description: "Regex for classes to exclude from this rule.",
				Apply:       func(r *FunctionNamingRule, v *regexp.Regexp) { r.ExcludeClassPattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[FunctionNamingRule]{
				Name:        "functionPattern",
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for function names.",
				Apply:       func(r *FunctionNamingRule, v *regexp.Regexp) { r.Pattern = v },
			}),
			api.StringListOption(api.StringListOptionSpec[FunctionNamingRule]{
				Name:        "ignoreAnnotated",
				Description: "Annotations that suppress this rule.",
				Apply:       func(r *FunctionNamingRule, v []string) { r.IgnoreAnnotated = v },
			}),
			api.BoolOption(api.BoolOptionSpec[FunctionNamingRule]{
				Name:        "customPreviewWildcard",
				Default:     true,
				Description: "Treat annotations whose simple name ends with Preview/Previews as preview scaffolding.",
				Apply:       func(r *FunctionNamingRule, v bool) { r.CustomPreviewWildcard = v },
			}),
			api.StringListOption(api.StringListOptionSpec[FunctionNamingRule]{
				Name:        "customPreviewPrefixes",
				Description: "Annotation prefixes to treat as preview scaffolding, e.g. Custom matches CustomPreview.",
				Apply:       func(r *FunctionNamingRule, v []string) { r.CustomPreviewPrefixes = v },
			}),
		},
	}
}

func (r *FunctionParameterNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FunctionParameterNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[FunctionParameterNamingRule]{
				Name:        "excludeClassPattern",
				Default:     "",
				Description: "Regex for classes to exclude from this rule.",
				Apply:       func(r *FunctionParameterNamingRule, v *regexp.Regexp) { r.ExcludeClassPattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[FunctionParameterNamingRule]{
				Name:        "parameterPattern",
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for function parameter names.",
				Apply:       func(r *FunctionParameterNamingRule, v *regexp.Regexp) { r.Pattern = v },
			}),
		},
	}
}

func (r *InvalidPackageDeclarationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InvalidPackageDeclaration",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[InvalidPackageDeclarationRule]{
				Name:        "requireRootInDeclaration",
				Default:     false,
				Description: "Require rootPackage in package declaration.",
				Apply:       func(r *InvalidPackageDeclarationRule, v bool) { r.RequireRootInDeclaration = v },
			}),
			api.StringOption(api.StringOptionSpec[InvalidPackageDeclarationRule]{
				Name:        "rootPackage",
				Default:     "",
				Description: "Root package prefix to require.",
				Apply:       func(r *InvalidPackageDeclarationRule, v string) { r.RootPackage = v },
			}),
		},
	}
}

func (r *LambdaParameterNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LambdaParameterNaming",
		RuleSet:       "naming",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[LambdaParameterNamingRule]{
				Name:        "parameterPattern",
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for lambda parameter names.",
				Apply:       func(r *LambdaParameterNamingRule, v *regexp.Regexp) { r.Pattern = v },
			}),
		},
	}
}

func (r *MatchingDeclarationNameRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MatchingDeclarationName",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[MatchingDeclarationNameRule]{
				Name:        "multiplatformTargets",
				Default:     []string{"ios", "android", "js", "jvm", "native", "iosArm64", "iosX64", "macosX64", "mingwX64", "linuxX64"},
				Description: "Multiplatform target suffixes to strip from filename.",
				Apply:       func(r *MatchingDeclarationNameRule, v []string) { r.MultiplatformTargets = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MatchingDeclarationNameRule]{
				Name:        "mustBeFirst",
				Default:     true,
				Description: "Declaration must be first in file.",
				Apply:       func(r *MatchingDeclarationNameRule, v bool) { r.MustBeFirst = v },
			}),
		},
	}
}

func (r *MemberNameEqualsClassNameRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MemberNameEqualsClassName",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[MemberNameEqualsClassNameRule]{
				Name:        "ignoreOverridden",
				Default:     true,
				Description: "Ignore overridden members.",
				Apply:       func(r *MemberNameEqualsClassNameRule, v bool) { r.IgnoreOverridden = v },
			}),
		},
	}
}

func (r *NoNameShadowingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NoNameShadowing",
		RuleSet:       "naming",
		DefaultActive: true,
	}
}

func (r *NonBooleanPropertyPrefixedWithIsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NonBooleanPropertyPrefixedWithIs",
		RuleSet:       "naming",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *ObjectPropertyNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ObjectPropertyNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[ObjectPropertyNamingRule]{
				Name:        "constantPattern",
				Default:     "[A-Z][_A-Z0-9]*",
				Description: "Regex pattern for constant properties in objects.",
				Apply:       func(r *ObjectPropertyNamingRule, v *regexp.Regexp) { r.ConstPattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[ObjectPropertyNamingRule]{
				Name:        "privatePropertyPattern",
				Default:     "",
				Description: "Regex for private properties in objects.",
				Apply:       func(r *ObjectPropertyNamingRule, v *regexp.Regexp) { r.PrivatePropertyPattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[ObjectPropertyNamingRule]{
				Name:        "propertyPattern",
				Default:     "[a-z][A-Za-z0-9]*",
				Description: "Regex pattern for non-constant properties in objects.",
				Apply:       func(r *ObjectPropertyNamingRule, v *regexp.Regexp) { r.PropertyPattern = v },
			}),
		},
	}
}

func (r *PackageNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PackageNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[PackageNamingRule]{
				Name:        "packagePattern",
				Default:     "[a-z]+(\\.[a-z][A-Za-z0-9]*)*",
				Description: "Regex pattern for package names.",
				Apply:       func(r *PackageNamingRule, v *regexp.Regexp) { r.Pattern = v },
			}),
		},
	}
}

func (r *TopLevelPropertyNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TopLevelPropertyNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[TopLevelPropertyNamingRule]{
				Name:        "constantPattern",
				Default:     "[A-Z][_A-Za-z0-9]*",
				Description: "Regex pattern for top-level constant properties.",
				Apply:       func(r *TopLevelPropertyNamingRule, v *regexp.Regexp) { r.ConstPattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[TopLevelPropertyNamingRule]{
				Name:        "privatePropertyPattern",
				Default:     "",
				Description: "Regex for private top-level properties.",
				Apply:       func(r *TopLevelPropertyNamingRule, v *regexp.Regexp) { r.PrivatePropertyPattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[TopLevelPropertyNamingRule]{
				Name:        "propertyPattern",
				Default:     "[a-z][A-Za-z0-9]*",
				Description: "Regex pattern for top-level non-constant properties.",
				Apply:       func(r *TopLevelPropertyNamingRule, v *regexp.Regexp) { r.PropertyPattern = v },
			}),
		},
	}
}

func (r *VariableMaxLengthRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VariableMaxLength",
		RuleSet:       "naming",
		DefaultActive: false,
		OptInReason:   api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[VariableMaxLengthRule]{
				Name:        "maximumVariableNameLength",
				Aliases:     []string{"maxLength"},
				Default:     64,
				Description: "Maximum allowed variable name length.",
				Apply:       func(r *VariableMaxLengthRule, v int) { r.MaxLength = v },
			}),
		},
	}
}

func (r *VariableMinLengthRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VariableMinLength",
		RuleSet:       "naming",
		DefaultActive: false,
		OptInReason:   api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[VariableMinLengthRule]{
				Name:        "minimumVariableNameLength",
				Aliases:     []string{"minLength"},
				Default:     2,
				Description: "Minimum allowed variable name length.",
				Apply:       func(r *VariableMinLengthRule, v int) { r.MinLength = v },
			}),
		},
	}
}

func (r *VariableNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VariableNaming",
		RuleSet:       "naming",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[VariableNamingRule]{
				Name:        "excludeClassPattern",
				Default:     "",
				Description: "Regex for classes to exclude from this rule.",
				Apply:       func(r *VariableNamingRule, v *regexp.Regexp) { r.ExcludeClassPattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[VariableNamingRule]{
				Name:        "privateVariablePattern",
				Default:     "",
				Description: "Regex pattern for private variable names.",
				Apply:       func(r *VariableNamingRule, v *regexp.Regexp) { r.PrivateVariablePattern = v },
			}),
			api.RegexOption(api.RegexOptionSpec[VariableNamingRule]{
				Name:        "variablePattern",
				Default:     "[a-z][a-zA-Z0-9]*",
				Description: "Regex pattern for variable names.",
				Apply:       func(r *VariableNamingRule, v *regexp.Regexp) { r.Pattern = v },
			}),
		},
	}
}
