// Descriptor metadata for internal/rules/potentialbugs_misc.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *DeprecationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Deprecation",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[DeprecationRule]{
				Name:        "excludeImportStatements",
				Default:     false,
				Description: "Exclude references inside import statements from deprecation checks.",
				Apply:       func(r *DeprecationRule, v bool) { r.ExcludeImportStatements = v },
			}),
		},
	}
}

func (r *HasPlatformTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HasPlatformType",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *IgnoredReturnValueRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IgnoredReturnValue",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[IgnoredReturnValueRule]{
				Name:        "ignoreFunctionCall",
				Description: "Function calls to ignore.",
				Apply:       func(r *IgnoredReturnValueRule, v []string) { r.IgnoreFunctionCall = v },
			}),
			api.StringListOption(api.StringListOptionSpec[IgnoredReturnValueRule]{
				Name:        "ignoreReturnValueAnnotations",
				Description: "Annotations that override return value checking.",
				Apply:       func(r *IgnoredReturnValueRule, v []string) { r.IgnoreReturnValueAnnotations = v },
			}),
			api.BoolOption(api.BoolOptionSpec[IgnoredReturnValueRule]{
				Name:        "restrictToConfig",
				Default:     false,
				Description: "Only check configured return types/annotations.",
				Apply:       func(r *IgnoredReturnValueRule, v bool) { r.RestrictToConfig = v },
			}),
			api.StringListOption(api.StringListOptionSpec[IgnoredReturnValueRule]{
				Name:        "returnValueAnnotations",
				Description: "Annotations indicating return value must be used.",
				Apply:       func(r *IgnoredReturnValueRule, v []string) { r.ReturnValueAnnotations = v },
			}),
			api.StringListOption(api.StringListOptionSpec[IgnoredReturnValueRule]{
				Name:        "returnValueTypes",
				Description: "Types whose return values must be used.",
				Apply:       func(r *IgnoredReturnValueRule, v []string) { r.ReturnValueTypes = v },
			}),
		},
	}
}

func (r *ImplicitDefaultLocaleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ImplicitDefaultLocale",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *LocaleDefaultForCurrencyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LocaleDefaultForCurrency",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *HardcodedDateFormatRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedDateFormat",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}

func (r *HardcodedNumberFormatRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedNumberFormat",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
	}
}
