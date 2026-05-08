// Descriptor metadata for internal/rules/comments.go.

package rules

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var _ = regexp.MustCompile

func (r *AbsentOrWrongFileLicenseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AbsentOrWrongFileLicense",
		RuleSet:       "comments",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Options: []api.ConfigOption{
			api.StringOption(api.StringOptionSpec[AbsentOrWrongFileLicenseRule]{
				Name:        "licenseTemplate",
				Default:     "Copyright",
				Description: "License header text (or regex) that every file must contain.",
				Apply:       func(r *AbsentOrWrongFileLicenseRule, v string) { r.LicenseTemplate = v },
			}),
			api.BoolOption(api.BoolOptionSpec[AbsentOrWrongFileLicenseRule]{
				Name:        "licenseTemplateIsRegex",
				Default:     false,
				Description: "Whether licenseTemplate is a regex.",
				Apply:       func(r *AbsentOrWrongFileLicenseRule, v bool) { r.IsRegex = v },
			}),
		},
	}
}

func (r *DeprecatedBlockTagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DeprecatedBlockTag",
		RuleSet:       "comments",
		DefaultActive: false,
	}
}

func (r *DocumentationOverPrivateFunctionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DocumentationOverPrivateFunction",
		RuleSet:       "comments",
		DefaultActive: false,
		FixLevel:      "cosmetic",
	}
}

func (r *DocumentationOverPrivatePropertyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DocumentationOverPrivateProperty",
		RuleSet:       "comments",
		DefaultActive: false,
		FixLevel:      "cosmetic",
	}
}

func (r *EndOfSentenceFormatRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EndOfSentenceFormat",
		RuleSet:       "comments",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.RegexOption(api.RegexOptionSpec[EndOfSentenceFormatRule]{
				Name:        "endOfSentenceFormat",
				Default:     `([.?!][ \t\n\r])|([.?!]$)`,
				Description: "Regex pattern matched against the first KDoc sentence's terminator.",
				Apply:       func(r *EndOfSentenceFormatRule, v *regexp.Regexp) { r.Pattern = v },
			}),
		},
	}
}

func (r *KDocReferencesNonPublicPropertyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "KDocReferencesNonPublicProperty",
		RuleSet:       "comments",
		DefaultActive: false,
	}
}

func (r *SampleAnnotationFreshnessRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SampleAnnotationFreshness",
		RuleSet:       "comments",
		DefaultActive: false,
	}
}

func (r *KdocLinkValidationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "KdocLinkValidation",
		RuleSet:       "comments",
		DefaultActive: false,
	}
}

func (r *OutdatedDocumentationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OutdatedDocumentation",
		RuleSet:       "comments",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[OutdatedDocumentationRule]{
				Name:        "matchDeclarationsOrder",
				Default:     false,
				Description: "Check that @param order matches declaration order.",
				Apply:       func(r *OutdatedDocumentationRule, v bool) { r.MatchDeclarationsOrder = v },
			}),
			api.BoolOption(api.BoolOptionSpec[OutdatedDocumentationRule]{
				Name:        "matchTypeParameters",
				Default:     false,
				Description: "Also check @param for type parameters.",
				Apply:       func(r *OutdatedDocumentationRule, v bool) { r.MatchTypeParameters = v },
			}),
		},
	}
}

func (r *UndocumentedPublicClassRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UndocumentedPublicClass",
		RuleSet:       "comments",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[UndocumentedPublicClassRule]{
				Name:        "searchInInnerClass",
				Default:     true,
				Description: "Also check inner classes.",
				Apply:       func(r *UndocumentedPublicClassRule, v bool) { r.SearchInInnerClass = v },
			}),
			api.BoolOption(api.BoolOptionSpec[UndocumentedPublicClassRule]{
				Name:        "searchInInnerInterface",
				Default:     true,
				Description: "Also check inner interfaces.",
				Apply:       func(r *UndocumentedPublicClassRule, v bool) { r.SearchInInnerInterface = v },
			}),
			api.BoolOption(api.BoolOptionSpec[UndocumentedPublicClassRule]{
				Name:        "searchInInnerObject",
				Default:     true,
				Description: "Also check inner objects.",
				Apply:       func(r *UndocumentedPublicClassRule, v bool) { r.SearchInInnerObject = v },
			}),
			api.BoolOption(api.BoolOptionSpec[UndocumentedPublicClassRule]{
				Name:        "searchInNestedClass",
				Default:     true,
				Description: "Also check nested classes.",
				Apply:       func(r *UndocumentedPublicClassRule, v bool) { r.SearchInNestedClass = v },
			}),
		},
	}
}

func (r *UndocumentedPublicFunctionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UndocumentedPublicFunction",
		RuleSet:       "comments",
		DefaultActive: false,
	}
}

func (r *UndocumentedPublicPropertyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UndocumentedPublicProperty",
		RuleSet:       "comments",
		DefaultActive: false,
	}
}
