// Descriptor metadata for internal/rules/style_format.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *CascadingCallWrappingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CascadingCallWrapping",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects chained calls that are not properly indented from the previous line.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "includeElvis",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Also check elvis operator chaining.",
				Apply: func(target interface{}, value interface{}) {
					target.(*CascadingCallWrappingRule).IncludeElvis = value.(bool)
				},
			},
		},
	}
}

func (r *EqualsOnSignatureLineRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "EqualsOnSignatureLine",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects expression body equals signs placed on a separate line from the function signature.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
	}
}

func (r *MaxChainedCallsOnSameLineRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MaxChainedCallsOnSameLine",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects lines with more chained method calls than the configured maximum.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "maxChainedCalls",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     5,
				Description: "Maximum chained calls allowed on one line.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MaxChainedCallsOnSameLineRule).MaxCalls = value.(int)
				},
			},
		},
	}
}

func (r *MaxLineLengthRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MaxLineLength",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects lines that exceed the configured maximum character length.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "excludeCommentStatements",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Exclude comment lines from check.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MaxLineLengthRule).ExcludeCommentStatements = value.(bool)
				},
			},
			{
				Name:        "excludeImportStatements",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Exclude import statements from check.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MaxLineLengthRule).ExcludeImportStatements = value.(bool)
				},
			},
			{
				Name:        "excludePackageStatements",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Exclude package statements from check.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MaxLineLengthRule).ExcludePackageStatements = value.(bool)
				},
			},
			{
				Name:        "excludeRawStrings",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Exclude raw string lines from check.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MaxLineLengthRule).ExcludeRawStrings = value.(bool)
				},
			},
			{
				Name:        "maxLineLength",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     120,
				Description: "Maximum allowed line length.",
				Apply:       func(target interface{}, value interface{}) { target.(*MaxLineLengthRule).Max = value.(int) },
			},
		},
	}
}

func (r *NewLineAtEndOfFileRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NewLineAtEndOfFile",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects files that do not end with a newline character.",
		DefaultActive: true,
		FixLevel:      "cosmetic",
		Confidence:    0.95,
	}
}

func (r *NoTabsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NoTabs",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects tab characters used for indentation instead of spaces.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.95,
		Options: []v2.ConfigOption{
			{
				Name:        "indentSize",
				Aliases:     []string{"indent_size", "tabWidth", "tab_width"},
				Type:        v2.OptInt,
				Default:     4,
				Description: "Number of spaces to replace each tab with when applying the fix.",
				Apply: func(target interface{}, value interface{}) {
					target.(*NoTabsRule).IndentSize = value.(int)
				},
			},
		},
	}
}

func (r *SpacingAfterPackageAndImportsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SpacingAfterPackageAndImports",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects missing blank lines after package and import declarations.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.95,
	}
}

func (r *TrailingWhitespaceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TrailingWhitespace",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects lines that end with trailing whitespace characters.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.95,
	}
}

func (r *UnderscoresInNumericLiteralsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnderscoresInNumericLiterals",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects large numeric literals that should use underscore separators for readability.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "acceptableLength",
				Type:        v2.OptInt,
				Default:     4,
				Description: "Acceptable numeric literal length without underscores.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnderscoresInNumericLiteralsRule).AcceptableLength = value.(int)
				},
			},
			{
				Name:        "allowNonStandardGrouping",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Allow underscore groups that are not exactly three digits.",
				Apply: func(target interface{}, value interface{}) {
					target.(*UnderscoresInNumericLiteralsRule).AllowNonStandardGrouping = value.(bool)
				},
			},
		},
	}
}
