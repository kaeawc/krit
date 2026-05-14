// Descriptor metadata for internal/rules/complexity.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *CognitiveComplexMethodRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CognitiveComplexMethod",
		RuleSet:       "complexity",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[CognitiveComplexMethodRule]{
				Name:        "allowedComplexity",
				Aliases:     []string{"threshold"},
				Default:     15,
				Description: "Maximum allowed cognitive complexity.",
				Apply:       func(r *CognitiveComplexMethodRule, v int) { r.AllowedComplexity = v },
			}),
		},
	}
}

func (r *ComplexConditionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComplexCondition",
		RuleSet:       "complexity",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[ComplexConditionRule]{
				Name:        "allowedConditions",
				Aliases:     []string{"threshold"},
				Default:     3,
				Description: "Maximum allowed conditions in an expression.",
				Apply:       func(r *ComplexConditionRule, v int) { r.AllowedConditions = v },
			}),
		},
	}
}

func (r *ComplexInterfaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComplexInterface",
		RuleSet:       "complexity",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[ComplexInterfaceRule]{
				Name:        "allowedDefinitions",
				Aliases:     []string{"threshold"},
				Default:     10,
				Description: "Maximum allowed definitions in an interface.",
				Apply:       func(r *ComplexInterfaceRule, v int) { r.AllowedDefinitions = v },
			}),
			api.BoolOption(api.BoolOptionSpec[ComplexInterfaceRule]{
				Name:        "includePrivateDeclarations",
				Default:     false,
				Description: "Count private declarations.",
				Apply:       func(r *ComplexInterfaceRule, v bool) { r.IncludePrivateDeclarations = v },
			}),
			api.BoolOption(api.BoolOptionSpec[ComplexInterfaceRule]{
				Name:        "includeStaticDeclarations",
				Default:     false,
				Description: "Count static/companion declarations.",
				Apply:       func(r *ComplexInterfaceRule, v bool) { r.IncludeStaticDeclarations = v },
			}),
		},
	}
}

func (r *CyclomaticComplexMethodRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CyclomaticComplexMethod",
		RuleSet:       "complexity",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[CyclomaticComplexMethodRule]{
				Name:        "allowedComplexity",
				Aliases:     []string{"threshold"},
				Default:     14,
				Description: "Maximum allowed cyclomatic complexity.",
				Apply:       func(r *CyclomaticComplexMethodRule, v int) { r.AllowedComplexity = v },
			}),
			api.BoolOption(api.BoolOptionSpec[CyclomaticComplexMethodRule]{
				Name:        "ignoreLocalFunctions",
				Default:     false,
				Description: "Ignore local functions.",
				Apply:       func(r *CyclomaticComplexMethodRule, v bool) { r.IgnoreLocalFunctions = v },
			}),
			api.BoolOption(api.BoolOptionSpec[CyclomaticComplexMethodRule]{
				Name:        "ignoreNestingFunctions",
				Default:     false,
				Description: "Ignore nesting functions.",
				Apply:       func(r *CyclomaticComplexMethodRule, v bool) { r.IgnoreNestingFunctions = v },
			}),
			api.BoolOption(api.BoolOptionSpec[CyclomaticComplexMethodRule]{
				Name:        "ignoreSimpleWhenEntries",
				Default:     false,
				Description: "Ignore simple when entries.",
				Apply:       func(r *CyclomaticComplexMethodRule, v bool) { r.IgnoreSimpleWhenEntries = v },
			}),
			api.BoolOption(api.BoolOptionSpec[CyclomaticComplexMethodRule]{
				Name:        "ignoreSingleWhenExpression",
				Default:     false,
				Description: "Ignore single when expressions.",
				Apply:       func(r *CyclomaticComplexMethodRule, v bool) { r.IgnoreSingleWhenExpression = v },
			}),
			api.StringListOption(api.StringListOptionSpec[CyclomaticComplexMethodRule]{
				Name:        "nestingFunctions",
				Description: "Functions treated as nesting.",
				Apply:       func(r *CyclomaticComplexMethodRule, v []string) { r.NestingFunctions = v },
			}),
		},
	}
}

func (r *LabeledExpressionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LabeledExpression",
		RuleSet:       "complexity",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[LabeledExpressionRule]{
				Name:        "ignoredLabels",
				Description: "Label names to ignore.",
				Apply:       func(r *LabeledExpressionRule, v []string) { r.IgnoredLabels = v },
			}),
		},
	}
}

func (r *LargeClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LargeClass",
		RuleSet:       "complexity",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[LargeClassRule]{
				Name:        "allowedLines",
				Aliases:     []string{"threshold"},
				Default:     600,
				Description: "Maximum allowed lines in a class.",
				Apply:       func(r *LargeClassRule, v int) { r.AllowedLines = v },
			}),
		},
	}
}

func (r *LongMethodRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LongMethod",
		RuleSet:       "complexity",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[LongMethodRule]{
				Name:        "allowedLines",
				Aliases:     []string{"threshold"},
				Default:     60,
				Description: "Maximum allowed lines in a method.",
				Apply:       func(r *LongMethodRule, v int) { r.AllowedLines = v },
			}),
		},
	}
}

func (r *LongParameterListRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LongParameterList",
		RuleSet:       "complexity",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[LongParameterListRule]{
				Name:        "allowedConstructorParameters",
				Default:     6,
				Description: "Maximum allowed constructor parameters.",
				Apply:       func(r *LongParameterListRule, v int) { r.AllowedConstructorParameters = v },
			}),
			api.IntOption(api.IntOptionSpec[LongParameterListRule]{
				Name:        "allowedFunctionParameters",
				Aliases:     []string{"threshold"},
				Default:     5,
				Description: "Maximum allowed function parameters.",
				Apply:       func(r *LongParameterListRule, v int) { r.AllowedFunctionParameters = v },
			}),
			api.StringListOption(api.StringListOptionSpec[LongParameterListRule]{
				Name:        "ignoreAnnotatedParameter",
				Description: "Annotations that exclude a parameter from counting.",
				Apply:       func(r *LongParameterListRule, v []string) { r.IgnoreAnnotatedParameter = v },
			}),
			api.BoolOption(api.BoolOptionSpec[LongParameterListRule]{
				Name:        "ignoreDataClasses",
				Default:     true,
				Description: "Ignore data class constructors.",
				Apply:       func(r *LongParameterListRule, v bool) { r.IgnoreDataClasses = v },
			}),
			api.BoolOption(api.BoolOptionSpec[LongParameterListRule]{
				Name:        "ignoreDefaultParameters",
				Default:     false,
				Description: "Ignore parameters with default values.",
				Apply:       func(r *LongParameterListRule, v bool) { r.IgnoreDefaultParameters = v },
			}),
		},
	}
}

func (r *MethodOverloadingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MethodOverloading",
		RuleSet:       "complexity",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[MethodOverloadingRule]{
				Name:        "allowedOverloads",
				Aliases:     []string{"threshold"},
				Default:     6,
				Description: "Maximum allowed method overloads.",
				Apply:       func(r *MethodOverloadingRule, v int) { r.AllowedOverloads = v },
			}),
		},
	}
}

func (r *NamedArgumentsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NamedArguments",
		RuleSet:       "complexity",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[NamedArgumentsRule]{
				Name:        "allowedArguments",
				Aliases:     []string{"threshold"},
				Default:     3,
				Description: "Maximum allowed positional arguments.",
				Apply:       func(r *NamedArgumentsRule, v int) { r.AllowedArguments = v },
			}),
		},
	}
}

func (r *NestedBlockDepthRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NestedBlockDepth",
		RuleSet:       "complexity",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[NestedBlockDepthRule]{
				Name:        "allowedDepth",
				Aliases:     []string{"threshold"},
				Default:     4,
				Description: "Maximum allowed nesting depth.",
				Apply:       func(r *NestedBlockDepthRule, v int) { r.AllowedDepth = v },
			}),
		},
	}
}

func (r *NestedScopeFunctionsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NestedScopeFunctions",
		RuleSet:       "complexity",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[NestedScopeFunctionsRule]{
				Name:        "allowedDepth",
				Aliases:     []string{"threshold"},
				Default:     1,
				Description: "Maximum allowed nested scope function depth.",
				Apply:       func(r *NestedScopeFunctionsRule, v int) { r.AllowedDepth = v },
			}),
			api.StringListOption(api.StringListOptionSpec[NestedScopeFunctionsRule]{
				Name:        "functions",
				Description: "Scope function names to check.",
				Apply:       func(r *NestedScopeFunctionsRule, v []string) { r.Functions = v },
			}),
		},
	}
}

func (r *ReplaceSafeCallChainWithRunRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ReplaceSafeCallChainWithRun",
		RuleSet:       "complexity",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
	}
}

func (r *StringLiteralDuplicationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringLiteralDuplication",
		RuleSet:       "complexity",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[StringLiteralDuplicationRule]{
				Name:        "allowedDuplications",
				Aliases:     []string{"threshold"},
				Default:     2,
				Description: "Maximum allowed string literal duplications.",
				Apply:       func(r *StringLiteralDuplicationRule, v int) { r.AllowedDuplications = v },
			}),
			api.IntOption(api.IntOptionSpec[StringLiteralDuplicationRule]{
				Name:        "allowedWithLengthLessThan",
				Default:     5,
				Description: "Strings whose unquoted content is shorter than this are ignored. Set to 0 to disable.",
				Apply:       func(r *StringLiteralDuplicationRule, v int) { r.AllowedWithLengthLessThan = v },
			}),
			api.BoolOption(api.BoolOptionSpec[StringLiteralDuplicationRule]{
				Name:        "ignoreAnnotation",
				Default:     false,
				Description: "Ignore strings in annotations.",
				Apply:       func(r *StringLiteralDuplicationRule, v bool) { r.IgnoreAnnotation = v },
			}),
			api.StringOption(api.StringOptionSpec[StringLiteralDuplicationRule]{
				Name:        "ignoreStringsRegex",
				Default:     "",
				Description: "Strings matching this regex are ignored.",
				Apply:       func(r *StringLiteralDuplicationRule, v string) { r.IgnoreStringsRegex = v },
			}),
		},
	}
}

func (r *TooManyFunctionsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TooManyFunctions",
		RuleSet:       "complexity",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[TooManyFunctionsRule]{
				Name:        "allowedFunctionsPerClass",
				Default:     0,
				Description: "Maximum allowed functions per class (0 = use file limit).",
				Apply:       func(r *TooManyFunctionsRule, v int) { r.AllowedFunctionsPerClass = v },
			}),
			api.IntOption(api.IntOptionSpec[TooManyFunctionsRule]{
				Name:        "allowedFunctionsPerEnum",
				Default:     0,
				Description: "Maximum allowed functions per enum (0 = use file limit).",
				Apply:       func(r *TooManyFunctionsRule, v int) { r.AllowedFunctionsPerEnum = v },
			}),
			api.IntOption(api.IntOptionSpec[TooManyFunctionsRule]{
				Name:        "allowedFunctionsPerFile",
				Aliases:     []string{"threshold"},
				Default:     11,
				Description: "Maximum allowed functions per file.",
				Apply:       func(r *TooManyFunctionsRule, v int) { r.AllowedFunctionsPerFile = v },
			}),
			api.IntOption(api.IntOptionSpec[TooManyFunctionsRule]{
				Name:        "allowedFunctionsPerInterface",
				Default:     0,
				Description: "Maximum allowed functions per interface (0 = use file limit).",
				Apply:       func(r *TooManyFunctionsRule, v int) { r.AllowedFunctionsPerInterface = v },
			}),
			api.IntOption(api.IntOptionSpec[TooManyFunctionsRule]{
				Name:        "allowedFunctionsPerObject",
				Default:     0,
				Description: "Maximum allowed functions per object (0 = use file limit).",
				Apply:       func(r *TooManyFunctionsRule, v int) { r.AllowedFunctionsPerObject = v },
			}),
			api.BoolOption(api.BoolOptionSpec[TooManyFunctionsRule]{
				Name:        "ignoreDeprecated",
				Default:     false,
				Description: "Ignore deprecated functions when counting.",
				Apply:       func(r *TooManyFunctionsRule, v bool) { r.IgnoreDeprecated = v },
			}),
			api.BoolOption(api.BoolOptionSpec[TooManyFunctionsRule]{
				Name:        "ignoreInternal",
				Default:     false,
				Description: "Ignore internal functions when counting.",
				Apply:       func(r *TooManyFunctionsRule, v bool) { r.IgnoreInternal = v },
			}),
			api.BoolOption(api.BoolOptionSpec[TooManyFunctionsRule]{
				Name:        "ignoreOverridden",
				Default:     false,
				Description: "Ignore overridden functions when counting.",
				Apply:       func(r *TooManyFunctionsRule, v bool) { r.IgnoreOverridden = v },
			}),
			api.BoolOption(api.BoolOptionSpec[TooManyFunctionsRule]{
				Name:        "ignorePrivate",
				Default:     false,
				Description: "Ignore private functions when counting.",
				Apply:       func(r *TooManyFunctionsRule, v bool) { r.IgnorePrivate = v },
			}),
		},
	}
}
