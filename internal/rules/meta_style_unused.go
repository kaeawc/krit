// Descriptor metadata for internal/rules/style_unused.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var _ = regexp.MustCompile

func (r *UnusedImportRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedImport",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "idiomatic",
	}
}

func (r *UnusedParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedParameter",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[UnusedParameterRule]{
				Name:        "allowedNames",
				Default:     "^(ignored|expected|_)$",
				Description: "Regex pattern for parameter names to allow.",
				Apply:       func(r *UnusedParameterRule, v *regexp.Regexp) { r.AllowedNames = v },
			}),
		},
	}
}

func (r *UnusedPrivateClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedPrivateClass",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *UnusedPrivateFunctionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedPrivateFunction",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[UnusedPrivateFunctionRule]{
				Name:        "allowedNames",
				Default:     "",
				Description: "Regex pattern for function names to allow.",
				Apply:       func(r *UnusedPrivateFunctionRule, v *regexp.Regexp) { r.AllowedNames = v },
			}),
		},
	}
}

func (r *UnusedPrivateMemberRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedPrivateMember",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[UnusedPrivateMemberRule]{
				Name:        "allowedNames",
				Default:     "",
				Description: "Regex pattern for member names to allow.",
				Apply:       func(r *UnusedPrivateMemberRule, v *regexp.Regexp) { r.AllowedNames = v },
			}),
			api.StringListOption(api.StringListOptionSpec[UnusedPrivateMemberRule]{
				Name:        "ignoreAnnotated",
				Description: "Annotations that suppress this rule.",
				Apply:       func(r *UnusedPrivateMemberRule, v []string) { r.IgnoreAnnotated = v },
			}),
		},
	}
}

func (r *UnusedPrivatePropertyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedPrivateProperty",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[UnusedPrivatePropertyRule]{
				Name:        "allowedNames",
				Default:     "",
				Description: "Regex pattern for property names to allow.",
				Apply:       func(r *UnusedPrivatePropertyRule, v *regexp.Regexp) { r.AllowedNames = v },
			}),
		},
	}
}

func (r *UnusedVariableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedVariable",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[UnusedVariableRule]{
				Name:        "allowedNames",
				Default:     "^(ignored|_)$",
				Description: "Regex pattern for variable names to allow.",
				Apply:       func(r *UnusedVariableRule, v *regexp.Regexp) { r.AllowedNames = v },
			}),
		},
	}
}
