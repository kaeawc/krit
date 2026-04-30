// Descriptor metadata for internal/rules/i18n_plurals.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *PluralsBuiltWithIfElseRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PluralsBuiltWithIfElse",
		RuleSet:       "i18n",
		Severity:      "warning",
		Description:   "Detects manual pluralization built with if/else over count == 1 instead of getQuantityString / pluralStringResource.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PluralsMissingZeroRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PluralsMissingZero",
		RuleSet:       "i18n",
		Severity:      "info",
		Description:   "<plurals> in a CLDR zero-form locale is missing the zero quantity",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *StringResourcePlaceholderOrderRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringResourcePlaceholderOrder",
		RuleSet:       "i18n",
		Severity:      "warning",
		Description:   "Translation variants must keep positional format syntax (`%1$s`, `%2$s`) used by the default string.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.9,
	}
}
