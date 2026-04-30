// Descriptor metadata for internal/rules/android_manifest_i18n.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *LocaleConfigMissingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LocaleConfigMissing",
		RuleSet:       "android-lint",
		Severity:      "info",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
