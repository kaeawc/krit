// Descriptor metadata for internal/rules/i18n_markup.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *TranslatableMarkupMismatchRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TranslatableMarkupMismatch",
		RuleSet:       "i18n",
		Severity:      "warning",
		Description:   "<string> markup style differs across locale variants (HTML vs Markdown vs plain text).",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.8,
	}
}
