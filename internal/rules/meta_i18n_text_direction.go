// Descriptor metadata for internal/rules/i18n_text_direction.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *TextDirectionLiteralInStringRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TextDirectionLiteralInString",
		RuleSet:       "i18n",
		Severity:      "info",
		Description:   "Detects string literals that embed Unicode BIDI control characters instead of using a directional formatter.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.9,
	}
}
