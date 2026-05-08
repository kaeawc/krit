// Descriptor metadata for internal/rules/style_expressions_extra.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *CanBeNonNullableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CanBeNonNullable",
		RuleSet:       "style",
		DefaultActive: false,
	}
}

func (r *DestructuringDeclarationWithTooManyEntriesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DestructuringDeclarationWithTooManyEntries",
		RuleSet:       "style",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[DestructuringDeclarationWithTooManyEntriesRule]{
				Name:        "maxDestructuringEntries",
				Aliases:     []string{"threshold"},
				Default:     3,
				Description: "Maximum destructuring entries allowed.",
				Apply:       func(r *DestructuringDeclarationWithTooManyEntriesRule, v int) { r.MaxEntries = v },
			}),
		},
	}
}

func (r *DoubleNegativeExpressionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DoubleNegativeExpression",
		RuleSet:       "style",
		DefaultActive: false,
		FixLevel:      "idiomatic",
	}
}

func (r *DoubleNegativeLambdaRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DoubleNegativeLambda",
		RuleSet:       "style",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[DoubleNegativeLambdaRule]{
				Name:        "negativeFunctions",
				Description: "Functions treated as negative.",
				Apply:       func(r *DoubleNegativeLambdaRule, v []string) { r.NegativeFunctions = v },
			}),
		},
	}
}

func (r *MultilineLambdaItParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MultilineLambdaItParameter",
		RuleSet:       "style",
		DefaultActive: false,
	}
}

func (r *MultilineRawStringIndentationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MultilineRawStringIndentation",
		RuleSet:       "style",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[MultilineRawStringIndentationRule]{
				Name:        "indentSize",
				Default:     4,
				Description: "Expected indent size.",
				Apply:       func(r *MultilineRawStringIndentationRule, v int) { r.IndentSize = v },
			}),
			api.StringListOption(api.StringListOptionSpec[MultilineRawStringIndentationRule]{
				Name:        "trimmingMethods",
				Description: "Methods used for trimming.",
				Apply:       func(r *MultilineRawStringIndentationRule, v []string) { r.TrimmingMethods = v },
			}),
		},
	}
}

func (r *NullableBooleanCheckRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NullableBooleanCheck",
		RuleSet:       "style",
		DefaultActive: false,
		FixLevel:      "idiomatic",
	}
}

func (r *RangeUntilInsteadOfRangeToRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RangeUntilInsteadOfRangeTo",
		RuleSet:       "style",
		DefaultActive: false,
		FixLevel:      "idiomatic",
	}
}

func (r *StringShouldBeRawStringRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringShouldBeRawString",
		RuleSet:       "style",
		DefaultActive: false,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[StringShouldBeRawStringRule]{
				Name:        "maxEscapedCharacterCount",
				Aliases:     []string{"maxEscapes"},
				Default:     2,
				Description: "Maximum escaped characters before suggesting raw string.",
				Apply:       func(r *StringShouldBeRawStringRule, v int) { r.MaxEscapes = v },
			}),
		},
	}
}

func (r *TrimMultilineRawStringRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TrimMultilineRawString",
		RuleSet:       "style",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[TrimMultilineRawStringRule]{
				Name:        "trimmingMethods",
				Description: "Methods used for trimming.",
				Apply:       func(r *TrimMultilineRawStringRule, v []string) { r.TrimmingMethods = v },
			}),
		},
	}
}
