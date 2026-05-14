// Descriptor metadata for internal/rules/android_manifest_i18n.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *LocaleConfigMissingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LocaleConfigMissing",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}
