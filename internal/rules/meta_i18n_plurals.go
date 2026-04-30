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
