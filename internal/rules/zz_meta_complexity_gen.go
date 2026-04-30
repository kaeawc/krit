// Descriptor metadata for internal/rules/complexity.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *CognitiveComplexMethodRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CognitiveComplexMethod",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects functions whose cognitive complexity exceeds a configurable threshold, weighting nesting depth.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedComplexity",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     15,
				Description: "Maximum allowed cognitive complexity.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CognitiveComplexMethodRule).AllowedComplexity = value.(int)
				},
			},
		},
	}
}

func (r *ComplexConditionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ComplexCondition",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects conditions with too many mixed logical operators.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedConditions",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     3,
				Description: "Maximum allowed conditions in an expression.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ComplexConditionRule).AllowedConditions = value.(int)
				},
			},
		},
	}
}

func (r *ComplexInterfaceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ComplexInterface",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects interfaces with too many member declarations.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedDefinitions",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     10,
				Description: "Maximum allowed definitions in an interface.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ComplexInterfaceRule).AllowedDefinitions = value.(int)
				},
			},
			{
				Name:        "includePrivateDeclarations",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Count private declarations.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ComplexInterfaceRule).IncludePrivateDeclarations = value.(bool)
				},
			},
			{
				Name:        "includeStaticDeclarations",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Count static/companion declarations.",
				Apply: func(target interface{}, value interface{}) {
					target.(*ComplexInterfaceRule).IncludeStaticDeclarations = value.(bool)
				},
			},
		},
	}
}

func (r *CyclomaticComplexMethodRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CyclomaticComplexMethod",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects functions whose cyclomatic complexity exceeds a configurable threshold.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedComplexity",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     14,
				Description: "Maximum allowed cyclomatic complexity.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CyclomaticComplexMethodRule).AllowedComplexity = value.(int)
				},
			},
			{
				Name:        "ignoreLocalFunctions",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore local functions.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CyclomaticComplexMethodRule).IgnoreLocalFunctions = value.(bool)
				},
			},
			{
				Name:        "ignoreNestingFunctions",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore nesting functions.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CyclomaticComplexMethodRule).IgnoreNestingFunctions = value.(bool)
				},
			},
			{
				Name:        "ignoreSimpleWhenEntries",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore simple when entries.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CyclomaticComplexMethodRule).IgnoreSimpleWhenEntries = value.(bool)
				},
			},
			{
				Name:        "ignoreSingleWhenExpression",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore single when expressions.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CyclomaticComplexMethodRule).IgnoreSingleWhenExpression = value.(bool)
				},
			},
			{
				Name:        "nestingFunctions",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Functions treated as nesting.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CyclomaticComplexMethodRule).NestingFunctions = value.([]string)
				},
			},
		},
	}
}

func (r *LabeledExpressionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LabeledExpression",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects labeled expressions such as return@label, break@label, and continue@label.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "ignoredLabels",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Label names to ignore.",
				Apply: func(target interface{}, value interface{}) {
					target.(*LabeledExpressionRule).IgnoredLabels = value.([]string)
				},
			},
		},
	}
}

func (r *LargeClassRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LargeClass",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects classes that exceed a configurable line count threshold.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedLines",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     600,
				Description: "Maximum allowed lines in a class.",
				Apply:       func(target interface{}, value interface{}) { target.(*LargeClassRule).AllowedLines = value.(int) },
			},
		},
	}
}

func (r *LongMethodRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LongMethod",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects functions that exceed a configurable line count threshold.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedLines",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     60,
				Description: "Maximum allowed lines in a method.",
				Apply:       func(target interface{}, value interface{}) { target.(*LongMethodRule).AllowedLines = value.(int) },
			},
		},
	}
}

func (r *LongParameterListRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LongParameterList",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects functions or constructors with too many parameters.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedConstructorParameters",
				Type:        v2.OptInt,
				Default:     6,
				Description: "Maximum allowed constructor parameters.",
				Apply: func(target interface{}, value interface{}) {
					target.(*LongParameterListRule).AllowedConstructorParameters = value.(int)
				},
			},
			{
				Name:        "allowedFunctionParameters",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     5,
				Description: "Maximum allowed function parameters.",
				Apply: func(target interface{}, value interface{}) {
					target.(*LongParameterListRule).AllowedFunctionParameters = value.(int)
				},
			},
			{
				Name:        "ignoreAnnotatedParameter",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Annotations that exclude a parameter from counting.",
				Apply: func(target interface{}, value interface{}) {
					target.(*LongParameterListRule).IgnoreAnnotatedParameter = value.([]string)
				},
			},
			{
				Name:        "ignoreDataClasses",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Ignore data class constructors.",
				Apply: func(target interface{}, value interface{}) {
					target.(*LongParameterListRule).IgnoreDataClasses = value.(bool)
				},
			},
			{
				Name:        "ignoreDefaultParameters",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore parameters with default values.",
				Apply: func(target interface{}, value interface{}) {
					target.(*LongParameterListRule).IgnoreDefaultParameters = value.(bool)
				},
			},
		},
	}
}

func (r *MethodOverloadingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MethodOverloading",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects methods with too many overloads of the same name in a scope.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedOverloads",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     6,
				Description: "Maximum allowed method overloads.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MethodOverloadingRule).AllowedOverloads = value.(int)
				},
			},
		},
	}
}

func (r *NamedArgumentsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NamedArguments",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects function calls with too many unnamed positional arguments.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedArguments",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     3,
				Description: "Maximum allowed positional arguments.",
				Apply: func(target interface{}, value interface{}) {
					target.(*NamedArgumentsRule).AllowedArguments = value.(int)
				},
			},
		},
	}
}

func (r *NestedBlockDepthRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NestedBlockDepth",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects functions with excessive nesting depth of control flow blocks.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedDepth",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     4,
				Description: "Maximum allowed nesting depth.",
				Apply:       func(target interface{}, value interface{}) { target.(*NestedBlockDepthRule).AllowedDepth = value.(int) },
			},
		},
	}
}

func (r *NestedScopeFunctionsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NestedScopeFunctions",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects excessively nested Kotlin scope functions like apply, also, let, run, and with.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedDepth",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     1,
				Description: "Maximum allowed nested scope function depth.",
				Apply: func(target interface{}, value interface{}) {
					target.(*NestedScopeFunctionsRule).AllowedDepth = value.(int)
				},
			},
			{
				Name:        "functions",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Scope function names to check.",
				Apply: func(target interface{}, value interface{}) {
					target.(*NestedScopeFunctionsRule).Functions = value.([]string)
				},
			},
		},
	}
}

func (r *ReplaceSafeCallChainWithRunRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ReplaceSafeCallChainWithRun",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects chains of three or more safe calls (?.) that could be simplified with ?.run { }.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringLiteralDuplicationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringLiteralDuplication",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects string literals that appear more than a configurable number of times in a file.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedDuplications",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     2,
				Description: "Maximum allowed string literal duplications.",
				Apply: func(target interface{}, value interface{}) {
					target.(*StringLiteralDuplicationRule).AllowedDuplications = value.(int)
				},
			},
			{
				Name:        "allowedWithLengthLessThan",
				Type:        v2.OptInt,
				Default:     5,
				Description: "Strings whose unquoted content is shorter than this are ignored. Set to 0 to disable.",
				Apply: func(target interface{}, value interface{}) {
					target.(*StringLiteralDuplicationRule).AllowedWithLengthLessThan = value.(int)
				},
			},
			{
				Name:        "ignoreAnnotation",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore strings in annotations.",
				Apply: func(target interface{}, value interface{}) {
					target.(*StringLiteralDuplicationRule).IgnoreAnnotation = value.(bool)
				},
			},
			{
				Name:        "ignoreStringsRegex",
				Type:        v2.OptString,
				Default:     "",
				Description: "Strings matching this regex are ignored.",
				Apply: func(target interface{}, value interface{}) {
					target.(*StringLiteralDuplicationRule).IgnoreStringsRegex = value.(string)
				},
			},
		},
	}
}

func (r *TooManyFunctionsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TooManyFunctions",
		RuleSet:       "complexity",
		Severity:      "warning",
		Description:   "Detects files or classes that declare too many functions.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedFunctionsPerClass",
				Type:        v2.OptInt,
				Default:     0,
				Description: "Maximum allowed functions per class (0 = use file limit).",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).AllowedFunctionsPerClass = value.(int)
				},
			},
			{
				Name:        "allowedFunctionsPerEnum",
				Type:        v2.OptInt,
				Default:     0,
				Description: "Maximum allowed functions per enum (0 = use file limit).",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).AllowedFunctionsPerEnum = value.(int)
				},
			},
			{
				Name:        "allowedFunctionsPerFile",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     11,
				Description: "Maximum allowed functions per file.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).AllowedFunctionsPerFile = value.(int)
				},
			},
			{
				Name:        "allowedFunctionsPerInterface",
				Type:        v2.OptInt,
				Default:     0,
				Description: "Maximum allowed functions per interface (0 = use file limit).",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).AllowedFunctionsPerInterface = value.(int)
				},
			},
			{
				Name:        "allowedFunctionsPerObject",
				Type:        v2.OptInt,
				Default:     0,
				Description: "Maximum allowed functions per object (0 = use file limit).",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).AllowedFunctionsPerObject = value.(int)
				},
			},
			{
				Name:        "ignoreDeprecated",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore deprecated functions when counting.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).IgnoreDeprecated = value.(bool)
				},
			},
			{
				Name:        "ignoreInternal",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore internal functions when counting.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).IgnoreInternal = value.(bool)
				},
			},
			{
				Name:        "ignoreOverridden",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore overridden functions when counting.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).IgnoreOverridden = value.(bool)
				},
			},
			{
				Name:        "ignorePrivate",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Ignore private functions when counting.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TooManyFunctionsRule).IgnorePrivate = value.(bool)
				},
			},
		},
	}
}
