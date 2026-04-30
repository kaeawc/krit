// Descriptor metadata for internal/rules/privacy_permissions.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AdMobInitializedBeforeConsentRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AdMobInitializedBeforeConsent",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects MobileAds.initialize() in Application.onCreate before any consent info update call.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *BiometricAuthNotFallingBackToDeviceCredentialRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "BiometricAuthNotFallingBackToDeviceCredential",
		RuleSet:       "privacy",
		Severity:      "info",
		Description:   "Detects BiometricPrompt.authenticate() calls whose PromptInfo lacks device credential fallback.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ClipboardOnSensitiveInputTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ClipboardOnSensitiveInputType",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects clipboard writes from variables whose names suggest passwords or credentials.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ContactsAccessWithoutPermissionUiRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ContactsAccessWithoutPermissionUi",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects contacts queries not gated behind a RequestPermission activity-result callback.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LocationBackgroundWithoutRationaleRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LocationBackgroundWithoutRationale",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects ACCESS_BACKGROUND_LOCATION requests without a shouldShowRequestPermissionRationale call.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ScreenshotNotBlockedOnLoginScreenRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ScreenshotNotBlockedOnLoginScreen",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects sensitive screens (login, payment, PIN) that do not set FLAG_SECURE to block screenshots.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
