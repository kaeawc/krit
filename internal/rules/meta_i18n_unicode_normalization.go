// Descriptor metadata for internal/rules/i18n_unicode_normalization.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *UnicodeNormalizationMissingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnicodeNormalizationMissing",
		RuleSet:       "i18n",
		Severity:      "info",
		Description:   "Detects contains() calls inside search/find functions that do not normalize operands; unicode-equivalent characters will not match.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.5,
	}
}
