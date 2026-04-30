// Descriptor metadata for internal/rules/comments.go.

package rules

import (
	"regexp"

	"github.com/kaeawc/krit/internal/rules/v2"
)

var _ = regexp.MustCompile

func (r *AbsentOrWrongFileLicenseRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AbsentOrWrongFileLicense",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects files that are missing a valid license header comment.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "licenseTemplate",
				Type:        v2.OptString,
				Default:     "Copyright",
				Description: "License header text (or regex) that every file must contain.",
				Apply: func(target interface{}, value interface{}) {
					target.(*AbsentOrWrongFileLicenseRule).LicenseTemplate = value.(string)
				},
			},
			{
				Name:        "licenseTemplateIsRegex",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Whether licenseTemplate is a regex.",
				Apply: func(target interface{}, value interface{}) {
					target.(*AbsentOrWrongFileLicenseRule).IsRegex = value.(bool)
				},
			},
		},
	}
}

func (r *DeprecatedBlockTagRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DeprecatedBlockTag",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects @deprecated KDoc tags that should use the @Deprecated annotation instead.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *DocumentationOverPrivateFunctionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DocumentationOverPrivateFunction",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects KDoc documentation on private functions where it is unnecessary.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.95,
	}
}

func (r *DocumentationOverPrivatePropertyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DocumentationOverPrivateProperty",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects KDoc documentation on private properties where it is unnecessary.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.95,
	}
}

func (r *EndOfSentenceFormatRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EndOfSentenceFormat",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects KDoc first sentences that do not end with proper punctuation.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "endOfSentenceFormat",
				Type:        v2.OptRegex,
				Default:     `([.?!][ \t\n\r])|([.?!]$)`,
				Description: "Regex pattern matched against the first KDoc sentence's terminator.",
				Apply: func(target interface{}, value interface{}) {
					target.(*EndOfSentenceFormatRule).Pattern = value.(*regexp.Regexp)
				},
			},
		},
	}
}

func (r *KDocReferencesNonPublicPropertyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "KDocReferencesNonPublicProperty",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects KDoc bracket references that point to non-public properties.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *OutdatedDocumentationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OutdatedDocumentation",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects @param tags in KDoc that do not match the actual function parameters.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "matchDeclarationsOrder",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Check that @param order matches declaration order.",
				Apply: func(target interface{}, value interface{}) {
					target.(*OutdatedDocumentationRule).MatchDeclarationsOrder = value.(bool)
				},
			},
			{
				Name:        "matchTypeParameters",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Also check @param for type parameters.",
				Apply: func(target interface{}, value interface{}) {
					target.(*OutdatedDocumentationRule).MatchTypeParameters = value.(bool)
				},
			},
		},
	}
}

func (r *UndocumentedPublicClassRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UndocumentedPublicClass",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects public classes that are missing KDoc documentation.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "searchInInnerClass",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Also check inner classes.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UndocumentedPublicClassRule).SearchInInnerClass = value.(bool)
				},
			},
			{
				Name:        "searchInInnerInterface",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Also check inner interfaces.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UndocumentedPublicClassRule).SearchInInnerInterface = value.(bool)
				},
			},
			{
				Name:        "searchInInnerObject",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Also check inner objects.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UndocumentedPublicClassRule).SearchInInnerObject = value.(bool)
				},
			},
			{
				Name:        "searchInNestedClass",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Also check nested classes.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UndocumentedPublicClassRule).SearchInNestedClass = value.(bool)
				},
			},
		},
	}
}

func (r *UndocumentedPublicFunctionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UndocumentedPublicFunction",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects public functions that are missing KDoc documentation.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *UndocumentedPublicPropertyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UndocumentedPublicProperty",
		RuleSet:       "comments",
		Severity:      "warning",
		Description:   "Detects public properties that are missing KDoc documentation.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
