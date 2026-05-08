// Descriptor metadata for internal/rules/privacy_storage.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *LogOfSharedPreferenceReadRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LogOfSharedPreferenceRead",
		RuleSet:       "privacy",
		DefaultActive: true,
	}
}

func (r *PlainFileWriteOfSensitiveRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PlainFileWriteOfSensitive",
		RuleSet:       "privacy",
		DefaultActive: true,
	}
}

func (r *SharedPreferencesForSensitiveKeyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SharedPreferencesForSensitiveKey",
		RuleSet:       "privacy",
		DefaultActive: true,
	}
}
