// Descriptor metadata for internal/rules/i18n_string_template.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *StringTemplateForTranslationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringTemplateForTranslation",
		RuleSet:       "i18n",
		Severity:      "info",
		Description:   "Detects string templates that embed stringResource(...) alongside another dynamic interpolation, which forces English word order.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
