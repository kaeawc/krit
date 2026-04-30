// Descriptor metadata for internal/rules/i18n_string_concat.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *StringConcatForTranslationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringConcatForTranslation",
		RuleSet:       "i18n",
		Severity:      "info",
		Description:   "Detects `+` concatenation between stringResource(...) and a non-literal, which forces English word order.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
