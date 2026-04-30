// Descriptor metadata for internal/rules/style_unused.go.

package rules

import (
	"regexp"

	"github.com/kaeawc/krit/internal/rules/v2"
)

var _ = regexp.MustCompile

func (r *UnusedImportRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedImport",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects import statements where the imported name is not referenced in the file.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *UnusedParameterRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedParameter",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects function parameters that are never used in the function body.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedNames",
				Type:        v2.OptRegex,
				Default:     "^(ignored|expected|_)$",
				Description: "Regex pattern for parameter names to allow.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnusedParameterRule).AllowedNames = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *UnusedPrivateClassRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedPrivateClass",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects private classes that are never referenced in the file.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
	}
}

func (r *UnusedPrivateFunctionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedPrivateFunction",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects private functions that are never called in the file.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedNames",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex pattern for function names to allow.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnusedPrivateFunctionRule).AllowedNames = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *UnusedPrivateMemberRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedPrivateMember",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects private members (classes, functions, properties) that are never used.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedNames",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex pattern for member names to allow.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnusedPrivateMemberRule).AllowedNames = value.(*regexp.Regexp)
				},
			},
			{
				Name:        "ignoreAnnotated",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Annotations that suppress this rule.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnusedPrivateMemberRule).IgnoreAnnotated = value.([]string)
				},
			},
		},
	}
}

func (r *UnusedPrivatePropertyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedPrivateProperty",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects private properties that are never referenced in the file.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedNames",
				Type:        v2.OptRegex,
				Default:     "",
				Description: "Regex pattern for property names to allow.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnusedPrivatePropertyRule).AllowedNames = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *UnusedVariableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedVariable",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects local variables that are declared but never used.",
		DefaultActive: true,
		FixLevel:      "semantic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedNames",
				Type:        v2.OptRegex,
				Default:     "^(ignored|_)$",
				Description: "Regex pattern for variable names to allow.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnusedVariableRule).AllowedNames = value.(*regexp.Regexp)
				},
			},
		},
	}
}
