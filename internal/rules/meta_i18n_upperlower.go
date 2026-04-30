// Descriptor metadata for internal/rules/i18n_upperlower.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *UpperLowerInvariantMisuseRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UpperLowerInvariantMisuse",
		RuleSet:       "i18n",
		Severity:      "warning",
		Description:   "Detects Kotlin 1.5+ uppercase()/lowercase() called without an explicit Locale argument.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
