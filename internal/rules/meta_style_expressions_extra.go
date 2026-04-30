// Descriptor metadata for internal/rules/style_expressions_extra.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *CanBeNonNullableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CanBeNonNullable",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects nullable types that are initialized with non-null values and never assigned null.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DestructuringDeclarationWithTooManyEntriesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DestructuringDeclarationWithTooManyEntries",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects destructuring declarations with more entries than the configured maximum.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "maxDestructuringEntries",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     3,
				Description: "Maximum destructuring entries allowed.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DestructuringDeclarationWithTooManyEntriesRule).MaxEntries = value.(int)
				},
			},
		},
	}
}

func (r *DoubleNegativeExpressionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DoubleNegativeExpression",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects double negative expressions like !isNotEmpty() that should use the positive variant.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.9,
	}
}

func (r *DoubleNegativeLambdaRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DoubleNegativeLambda",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects double negative lambda patterns like filterNot { !predicate } that should use filter.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.9,
		Options: []v2.ConfigOption{
			{
				Name:        "negativeFunctions",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Functions treated as negative.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DoubleNegativeLambdaRule).NegativeFunctions = value.([]string)
				},
			},
		},
	}
}

func (r *MultilineLambdaItParameterRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MultilineLambdaItParameter",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects multiline lambdas that use the implicit it parameter instead of naming it explicitly.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MultilineRawStringIndentationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MultilineRawStringIndentation",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects multiline raw strings that are missing trimIndent() or trimMargin() calls.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "indentSize",
				Type:        v2.OptInt,
				Default:     4,
				Description: "Expected indent size.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MultilineRawStringIndentationRule).IndentSize = value.(int)
				},
			},
			{
				Name:        "trimmingMethods",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Methods used for trimming.",
				Apply: func(target interface{}, value interface{}) {
					target.(*MultilineRawStringIndentationRule).TrimmingMethods = value.([]string)
				},
			},
		},
	}
}

func (r *NullableBooleanCheckRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NullableBooleanCheck",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects equality comparisons against Boolean literals like x == true on nullable Booleans.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *RangeUntilInsteadOfRangeToRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RangeUntilInsteadOfRangeTo",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects usage of the until infix function that can be replaced with the ..< range operator.",
		DefaultActive: false,
		FixLevel:      "idiomatic",
		Confidence:    0.75,
	}
}

func (r *StringShouldBeRawStringRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringShouldBeRawString",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects string literals with many escape characters that would be more readable as raw strings.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "maxEscapedCharacterCount",
				Aliases:     []string{"maxEscapes"},
				Type:        v2.OptInt,
				Default:     2,
				Description: "Maximum escaped characters before suggesting raw string.",
				Apply: func(target interface{}, value interface{}) {
					target.(*StringShouldBeRawStringRule).MaxEscapes = value.(int)
				},
			},
		},
	}
}

func (r *TrimMultilineRawStringRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TrimMultilineRawString",
		RuleSet:       "style",
		Severity:      "warning",
		Description:   "Detects multiline raw strings that should use trimIndent() or trimMargin() for proper indentation.",
		DefaultActive: false,
		FixLevel:      "cosmetic",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "trimmingMethods",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Methods used for trimming.",
				Apply: func(target interface{}, value interface{}) {
					target.(*TrimMultilineRawStringRule).TrimmingMethods = value.([]string)
				},
			},
		},
	}
}
