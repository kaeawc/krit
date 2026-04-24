// Descriptor metadata for internal/rules/potentialbugs_misc.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *DeprecationRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "Deprecation",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects usage of deprecated functions, classes, or properties.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []registry.ConfigOption{
			{
				Name:        "excludeImportStatements",
				Type:        registry.OptBool,
				Default:     false,
				Description: "Exclude references inside import statements from deprecation checks.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DeprecationRule).ExcludeImportStatements = value.(bool)
				},
			},
		},
	}
}

func (r *HasPlatformTypeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "HasPlatformType",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects public functions with expression bodies that lack an explicit return type, risking platform type exposure from Java interop.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *IgnoredReturnValueRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "IgnoredReturnValue",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects discarded return values from functional operations or @CheckReturnValue-annotated functions.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []registry.ConfigOption{
			{
				Name:        "ignoreFunctionCall",
				Type:        registry.OptStringList,
				Default:     []string(nil),
				Description: "Function calls to ignore.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).IgnoreFunctionCall = value.([]string)
				},
			},
			{
				Name:        "ignoreReturnValueAnnotations",
				Type:        registry.OptStringList,
				Default:     []string(nil),
				Description: "Annotations that override return value checking.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).IgnoreReturnValueAnnotations = value.([]string)
				},
			},
			{
				Name:        "restrictToConfig",
				Type:        registry.OptBool,
				Default:     false,
				Description: "Only check configured return types/annotations.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).RestrictToConfig = value.(bool)
				},
			},
			{
				Name:        "returnValueAnnotations",
				Type:        registry.OptStringList,
				Default:     []string(nil),
				Description: "Annotations indicating return value must be used.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).ReturnValueAnnotations = value.([]string)
				},
			},
			{
				Name:        "returnValueTypes",
				Type:        registry.OptStringList,
				Default:     []string(nil),
				Description: "Types whose return values must be used.",
				Apply: func(target interface{}, value interface{}) {
					target.(*IgnoredReturnValueRule).ReturnValueTypes = value.([]string)
				},
			},
		},
	}
}

func (r *ImplicitDefaultLocaleRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ImplicitDefaultLocale",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects locale-sensitive string methods called without an explicit Locale argument.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LocaleDefaultForCurrencyRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LocaleDefaultForCurrency",
		RuleSet:       "potential-bugs",
		Severity:      "warning",
		Description:   "Detects Currency.getInstance(Locale.getDefault()) in money-related classes where currency should come from business data.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
