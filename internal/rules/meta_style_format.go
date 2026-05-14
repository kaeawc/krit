// Descriptor metadata for internal/rules/style_format.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *CascadingCallWrappingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CascadingCallWrapping",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[CascadingCallWrappingRule]{
				Name:        "includeElvis",
				Default:     false,
				Description: "Also check elvis operator chaining.",
				Apply:       func(r *CascadingCallWrappingRule, v bool) { r.IncludeElvis = v },
			}),
		},
	}
}

func (r *EqualsOnSignatureLineRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "EqualsOnSignatureLine",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}

func (r *MaxChainedCallsOnSameLineRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MaxChainedCallsOnSameLine",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[MaxChainedCallsOnSameLineRule]{
				Name:        "maxChainedCalls",
				Aliases:     []string{"threshold"},
				Default:     5,
				Description: "Maximum chained calls allowed on one line.",
				Apply:       func(r *MaxChainedCallsOnSameLineRule, v int) { r.MaxCalls = v },
			}),
		},
	}
}

func (r *MaxLineLengthRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MaxLineLength",
		RuleSet:       "style",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[MaxLineLengthRule]{
				Name:        "excludeCommentStatements",
				Default:     false,
				Description: "Exclude comment lines from check.",
				Apply:       func(r *MaxLineLengthRule, v bool) { r.ExcludeCommentStatements = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MaxLineLengthRule]{
				Name:        "excludeImportStatements",
				Default:     true,
				Description: "Exclude import statements from check.",
				Apply:       func(r *MaxLineLengthRule, v bool) { r.ExcludeImportStatements = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MaxLineLengthRule]{
				Name:        "excludePackageStatements",
				Default:     true,
				Description: "Exclude package statements from check.",
				Apply:       func(r *MaxLineLengthRule, v bool) { r.ExcludePackageStatements = v },
			}),
			api.BoolOption(api.BoolOptionSpec[MaxLineLengthRule]{
				Name:        "excludeRawStrings",
				Default:     true,
				Description: "Exclude raw string lines from check.",
				Apply:       func(r *MaxLineLengthRule, v bool) { r.ExcludeRawStrings = v },
			}),
			api.IntOption(api.IntOptionSpec[MaxLineLengthRule]{
				Name:        "maxLineLength",
				Aliases:     []string{"threshold"},
				Default:     120,
				Description: "Maximum allowed line length.",
				Apply:       func(r *MaxLineLengthRule, v int) { r.Max = v },
			}),
		},
	}
}

func (r *NewLineAtEndOfFileRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NewLineAtEndOfFile",
		RuleSet:       "style",
		DefaultActive: true,
		FixLevel:      "cosmetic",
	}
}

func (r *NoTabsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NoTabs",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[NoTabsRule]{
				Name:        "indentSize",
				Aliases:     []string{"indent_size", "tabWidth", "tab_width"},
				Default:     4,
				Description: "Number of spaces to replace each tab with when applying the fix.",
				Apply:       func(r *NoTabsRule, v int) { r.IndentSize = v },
			}),
		},
	}
}

func (r *SpacingAfterPackageAndImportsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SpacingAfterPackageAndImports",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}

func (r *TrailingWhitespaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TrailingWhitespace",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}

func (r *UnderscoresInNumericLiteralsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnderscoresInNumericLiterals",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[UnderscoresInNumericLiteralsRule]{
				Name:        "acceptableLength",
				Default:     4,
				Description: "Acceptable numeric literal length without underscores.",
				Apply:       func(r *UnderscoresInNumericLiteralsRule, v int) { r.AcceptableLength = v },
			}),
			api.BoolOption(api.BoolOptionSpec[UnderscoresInNumericLiteralsRule]{
				Name:        "allowNonStandardGrouping",
				Default:     false,
				Description: "Allow underscore groups that are not exactly three digits.",
				Apply:       func(r *UnderscoresInNumericLiteralsRule, v bool) { r.AllowNonStandardGrouping = v },
			}),
		},
	}
}
