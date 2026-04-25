// Descriptor metadata for internal/rules/android_manifest_i18n.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *LocaleConfigMissingRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LocaleConfigMissing",
		RuleSet:       "android-lint",
		Severity:      "info",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
