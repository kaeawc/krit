// Descriptor metadata for internal/rules/privacy_storage.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *LogOfSharedPreferenceReadRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LogOfSharedPreferenceRead",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects logger calls that directly pass SharedPreferences values with sensitive keys.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PlainFileWriteOfSensitiveRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PlainFileWriteOfSensitive",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects plain-file writes to paths containing sensitive terms without using EncryptedFile.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SharedPreferencesForSensitiveKeyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SharedPreferencesForSensitiveKey",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects SharedPreferences put calls with key names matching sensitive patterns like token, password, or secret.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
