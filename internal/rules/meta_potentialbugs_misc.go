// Descriptor metadata for internal/rules/potentialbugs_misc.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *DeprecationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Deprecation",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects usage of deprecated functions, classes, or properties.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "excludeImportStatements",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Exclude references inside import statements from deprecation checks.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DeprecationRule).ExcludeImportStatements = value.(bool)
				},
			},
		},
	}
}

func (r *HasPlatformTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HasPlatformType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects public functions with expression bodies that lack an explicit return type, risking platform type exposure from Java interop.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *IgnoredReturnValueRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "IgnoredReturnValue",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects discarded return values from functional operations or @CheckReturnValue-annotated functions.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "ignoreFunctionCall",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Function calls to ignore.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).IgnoreFunctionCall = value.([]string)
				},
			},
			{
				Name:        "ignoreReturnValueAnnotations",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Annotations that override return value checking.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).IgnoreReturnValueAnnotations = value.([]string)
				},
			},
			{
				Name:        "restrictToConfig",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Only check configured return types/annotations.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).RestrictToConfig = value.(bool)
				},
			},
			{
				Name:        "returnValueAnnotations",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Annotations indicating return value must be used.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).ReturnValueAnnotations = value.([]string)
				},
			},
			{
				Name:        "returnValueTypes",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Types whose return values must be used.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).ReturnValueTypes = value.([]string)
				},
			},
		},
	}
}

func (r *ImplicitDefaultLocaleRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ImplicitDefaultLocale",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects locale-sensitive string methods called without an explicit Locale argument.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LocaleDefaultForCurrencyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LocaleDefaultForCurrency",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects Currency.getInstance(Locale.getDefault()) in money-related classes where currency should come from business data.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HardcodedDateFormatRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HardcodedDateFormat",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects DateTimeFormatter.ofPattern constructed without an explicit Locale.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *HardcodedNumberFormatRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HardcodedNumberFormat",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects DecimalFormat or NumberFormat.getInstance constructed without an explicit Locale.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}
